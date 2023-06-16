package timecraft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/timecraft/sdk"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
	"golang.org/x/exp/slices"
)

// Runner coordinates the execution of WebAssembly modules.
type Runner struct {
	ctx      context.Context
	registry *timemachine.Registry
	runtime  wazero.Runtime
}

// NewRunner creates a Runner.
func NewRunner(ctx context.Context, registry *timemachine.Registry, runtime wazero.Runtime) *Runner {
	return &Runner{
		ctx:      ctx,
		registry: registry,
		runtime:  runtime,
	}
}

// Prepare prepares a Module for execution.
func (r *Runner) Prepare(moduleSpec ModuleSpec) (*Module, error) {
	wasmPath := moduleSpec.Path
	wasmName := filepath.Base(wasmPath)
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("could not read wasm file '%s': %w", wasmPath, err)
	}

	wasmModule, err := r.runtime.CompileModule(r.ctx, wasmCode)
	if err != nil {
		return nil, err
	}

	builder := imports.NewBuilder().
		WithName(wasmName).
		WithArgs(moduleSpec.Args...).
		WithEnv(moduleSpec.Env...).
		WithDirs(moduleSpec.Dirs...).
		WithListens(moduleSpec.Listens...).
		WithDials(moduleSpec.Dials...).
		WithStdio(moduleSpec.Stdin, moduleSpec.Stdout, moduleSpec.Stderr).
		WithSocketsExtension(moduleSpec.Sockets, wasmModule)

	ctx, cancel := context.WithCancel(r.ctx)

	return &Module{
		ctx:        ctx,
		cancel:     cancel,
		moduleSpec: moduleSpec.Copy(),
		wasmName:   wasmName,
		wasmCode:   wasmCode,
		wasmModule: wasmModule,
		builder:    builder,
	}, nil
}

// PrepareLog initializes a log to record a trace of execution.
func (r *Runner) PrepareLog(mod *Module, log LogSpec) error {
	if mod.logSpec != nil {
		return errors.New("the module already has a log")
	}

	module, err := r.registry.CreateModule(mod.ctx, &format.Module{
		Code: mod.wasmCode,
	}, object.Tag{
		Name:  "timecraft.module.name",
		Value: mod.wasmModule.Name(),
	})
	if err != nil {
		return err
	}

	runtime, err := r.registry.CreateRuntime(mod.ctx, &format.Runtime{
		Runtime: "timecraft",
		Version: Version(),
	})
	if err != nil {
		return err
	}

	config, err := r.registry.CreateConfig(mod.ctx, &format.Config{
		Runtime: runtime,
		Modules: []*format.Descriptor{module},
		Args:    append([]string{mod.wasmName}, mod.moduleSpec.Args...),
		Env:     mod.moduleSpec.Env,
	})
	if err != nil {
		return err
	}

	process, err := r.registry.CreateProcess(mod.ctx, &format.Process{
		ID:        log.ProcessID,
		StartTime: log.StartTime,
		Config:    config,
	})
	if err != nil {
		return err
	}

	if err := r.registry.CreateLogManifest(mod.ctx, log.ProcessID, &format.Manifest{
		Process:   process,
		StartTime: log.StartTime,
	}); err != nil {
		return err
	}

	// TODO: create a writer that writes to many segments
	logSegment, err := r.registry.CreateLogSegment(mod.ctx, log.ProcessID, 0)
	if err != nil {
		return err
	}
	mod.cleanup = append(mod.cleanup, logSegment.Close)
	logWriter := timemachine.NewLogWriter(logSegment)
	recordWriter := timemachine.NewLogRecordWriter(logWriter, log.BatchSize, log.Compression)
	mod.cleanup = append(mod.cleanup, recordWriter.Flush)

	mod.recorder = func(s wasi.System) wasi.System {
		var b timemachine.RecordBuilder
		return wasicall.NewRecorder(s, func(id wasicall.SyscallID, syscallBytes []byte) {
			b.Reset(log.StartTime)
			b.SetTimestamp(time.Now())
			b.SetFunctionID(int(id))
			b.SetFunctionCall(syscallBytes)
			if err := recordWriter.WriteRecord(&b); err != nil {
				panic(err) // TODO: better error handling
			}
		})
	}

	mod.logSpec = &log

	return nil
}

// RunModule runs a prepared WebAssembly module.
func (r *Runner) RunModule(mod *Module) error {
	if mod.run {
		return errors.New("module is already running or already has run")
	}
	mod.run = true

	server := Server{
		Runner:  r,
		Module:  mod.moduleSpec,
		Log:     mod.logSpec,
		Version: Version(),
	}
	serverSocket := path.Join(os.TempDir(), fmt.Sprintf("timecraft.%s.sock", uuid.NewString()))
	defer os.Remove(serverSocket)

	serverListener, err := net.Listen("unix", serverSocket)
	if err != nil {
		return err
	}
	defer serverListener.Close()

	go func() {
		if err := server.Serve(serverListener); err != nil && !errors.Is(err, net.ErrClosed) {
			panic(err) // TODO: better error handling
		}
	}()

	// Wrap the wasi.System to make the gRPC server accessible via a virtual
	// socket, to trace system calls (if applicable), and to record system
	// calls to the log. The order is important here!
	var wrappers []func(wasi.System) wasi.System
	wrappers = append(wrappers, func(system wasi.System) wasi.System {
		return NewVirtualSocketsSystem(system, map[string]string{
			sdk.ServerSocket: serverSocket,
		})
	})
	if mod.moduleSpec.Trace != nil {
		wrappers = append(wrappers, func(system wasi.System) wasi.System {
			return wasi.Trace(mod.moduleSpec.Trace, system)
		})
	}
	if mod.recorder != nil {
		wrappers = append(wrappers, mod.recorder)
	}
	mod.builder = mod.builder.WithWrappers(wrappers...)

	var system wasi.System
	mod.ctx, system, err = mod.builder.Instantiate(mod.ctx, r.runtime)
	if err != nil {
		return err
	}
	defer system.Close(mod.ctx)

	return runModule(mod.ctx, r.runtime, mod.wasmModule)
}

// ModuleSpec is the details about what WebAssembly module to execute,
// how it should be initialized, and what its inputs are.
type ModuleSpec struct {
	// Path is the path of the WebAssembly module.
	Path string

	// Args are command-line arguments to pass to the module.
	Args []string

	// Env is the environment variables to pass to the module.
	Env []string

	// Dirs is a set of directories to make available to the module.
	Dirs []string

	// Listens is a set of listener sockets to make available to the module.
	Listens []string

	// Dials is a set of connection sockets to make available to the module.
	Dials []string

	// Sockets is the name of a sockets extension to use, or "auto" to
	// automatically detect the sockets extension.
	Sockets string

	// Stdio file descriptors.
	Stdin  int
	Stdout int
	Stderr int

	// Trace is an optional writer that receives a trace of system calls
	// made by the module.
	Trace io.Writer
}

// Copy creates a deep copy of the ModuleSpec.
func (s ModuleSpec) Copy() (copy ModuleSpec) {
	copy = s
	copy.Args = slices.Clone(s.Args)
	copy.Env = slices.Clone(s.Env)
	copy.Dirs = slices.Clone(s.Dirs)
	copy.Listens = slices.Clone(s.Listens)
	copy.Dials = slices.Clone(s.Dials)
	return
}

// LogSpec is details about a log that stores a trace of execution.
type LogSpec struct {
	ProcessID   uuid.UUID
	StartTime   time.Time
	Compression timemachine.Compression
	BatchSize   int
}

// Module is a WebAssembly module that's ready for execution.
type Module struct {
	ctx    context.Context
	cancel context.CancelFunc

	moduleSpec ModuleSpec
	wasmName   string
	wasmCode   []byte
	wasmModule wazero.CompiledModule
	builder    *imports.Builder

	logSpec  *LogSpec
	recorder func(wasi.System) wasi.System

	run     bool
	cleanup []func() error
}

// Close closes the module.
func (m *Module) Close() error {
	var errs []error
	if len(m.cleanup) > 0 {
		for i := len(m.cleanup) - 1; i >= 0; i-- {
			if err := m.cleanup[i](); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if err := m.wasmModule.Close(m.ctx); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func runModule(ctx context.Context, runtime wazero.Runtime, compiledModule wazero.CompiledModule) error {
	module, err := runtime.InstantiateModule(ctx, compiledModule, wazero.NewModuleConfig().
		WithStartFunctions())
	if err != nil {
		return err
	}
	defer module.Close(ctx)

	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		_, err := module.ExportedFunction("_start").Call(ctx)
		module.Close(ctx)
		cancel(err)
	}()

	<-ctx.Done()

	err = context.Cause(ctx)
	switch err {
	case context.Canceled, context.DeadlineExceeded:
		err = nil
	}

	switch e := err.(type) {
	case *sys.ExitError:
		switch exitCode := e.ExitCode(); exitCode {
		case 0:
			err = nil
		default:
			err = ExitError(exitCode)
		}
	}

	return err
}

// ExitError indicates a WebAssembly module exited with a non-zero exit code.
type ExitError uint32

func (e ExitError) Error() string {
	return fmt.Sprintf("module exited with code %d", uint32(e))
}
