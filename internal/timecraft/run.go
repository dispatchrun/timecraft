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

// Runner coordinates the execution of a WebAssembly modules.
type Runner struct {
	registry *timemachine.Registry
	runtime  wazero.Runtime
}

// NewRunner creates a Runner.
func NewRunner(registry *timemachine.Registry, runtime wazero.Runtime) *Runner {
	return &Runner{
		registry: registry,
		runtime:  runtime,
	}
}

// PrepareModule prepares a module for execution.
func (r *Runner) PrepareModule(ctx context.Context, spec ModuleSpec) (*PreparedModule, error) {
	wasmPath := spec.Path
	wasmName := filepath.Base(wasmPath)
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return nil, fmt.Errorf("could not read wasm file '%s': %w", wasmPath, err)
	}

	wasmModule, err := r.runtime.CompileModule(ctx, wasmCode)
	if err != nil {
		return nil, err
	}

	builder := imports.NewBuilder().
		WithName(wasmName).
		WithArgs(spec.Args...).
		WithEnv(spec.Env...).
		WithDirs(spec.Dirs...).
		WithListens(spec.Listens...).
		WithDials(spec.Dials...).
		WithStdio(spec.Stdin, spec.Stdout, spec.Stderr).
		WithSocketsExtension(spec.Sockets, wasmModule)

	return &PreparedModule{
		moduleSpec: spec.Copy(),
		wasmName:   wasmName,
		wasmCode:   wasmCode,
		wasmModule: wasmModule,
		builder:    builder,
	}, nil
}

func (r *Runner) PrepareRecorder(ctx context.Context, m *PreparedModule, startTime time.Time, c timemachine.Compression, batchSize int) (uuid.UUID, error) {
	if m.recorder != nil {
		return uuid.UUID{}, errors.New("a recorder has already been attached to the module")
	}

	processID := uuid.New()

	module, err := r.registry.CreateModule(ctx, &format.Module{
		Code: m.wasmCode,
	}, object.Tag{
		Name:  "timecraft.module.name",
		Value: m.wasmModule.Name(),
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	runtime, err := r.registry.CreateRuntime(ctx, &format.Runtime{
		Runtime: "timecraft",
		Version: Version(),
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	config, err := r.registry.CreateConfig(ctx, &format.Config{
		Runtime: runtime,
		Modules: []*format.Descriptor{module},
		Args:    append([]string{m.wasmName}, m.moduleSpec.Args...),
		Env:     m.moduleSpec.Env,
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	process, err := r.registry.CreateProcess(ctx, &format.Process{
		ID:        processID,
		StartTime: startTime,
		Config:    config,
	})
	if err != nil {
		return uuid.UUID{}, err
	}

	if err := r.registry.CreateLogManifest(ctx, processID, &format.Manifest{
		Process:   process,
		StartTime: startTime,
	}); err != nil {
		return uuid.UUID{}, err
	}

	// TODO: create a writer that writes to many segments
	logSegment, err := r.registry.CreateLogSegment(ctx, processID, 0)
	if err != nil {
		return uuid.UUID{}, err
	}
	m.cleanup = append(m.cleanup, logSegment.Close)
	logWriter := timemachine.NewLogWriter(logSegment)
	recordWriter := timemachine.NewLogRecordWriter(logWriter, batchSize, c)
	m.cleanup = append(m.cleanup, recordWriter.Flush)

	m.recorder = func(s wasi.System) wasi.System {
		var b timemachine.RecordBuilder
		return wasicall.NewRecorder(s, func(id wasicall.SyscallID, syscallBytes []byte) {
			b.Reset(startTime)
			b.SetTimestamp(time.Now())
			b.SetFunctionID(int(id))
			b.SetFunctionCall(syscallBytes)
			if err := recordWriter.WriteRecord(&b); err != nil {
				panic(err) // TODO: better error handling
			}
		})
	}

	m.logSpec = &LogSpec{
		ID:          processID,
		Compression: c,
		BatchSize:   batchSize,
	}

	return processID, nil
}

// Run runs a WebAssembly module.
func (r *Runner) Run(ctx context.Context, m *PreparedModule) error {
	server := Server{
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

	var wrappers []func(wasi.System) wasi.System
	wrappers = append(wrappers, func(system wasi.System) wasi.System {
		return NewVirtualSocketsSystem(system, map[string]string{
			sdk.ServerSocket: serverSocket,
		})
	})
	if m.trace != nil {
		wrappers = append(wrappers, func(system wasi.System) wasi.System {
			return wasi.Trace(m.trace, system)
		})
	}
	if m.recorder != nil {
		wrappers = append(wrappers, m.recorder)
	}

	m.builder = m.builder.WithWrappers(wrappers...)

	var system wasi.System
	ctx, system, err = m.builder.Instantiate(ctx, r.runtime)
	if err != nil {
		return err
	}
	defer system.Close(ctx)

	return runModule(ctx, r.runtime, m.wasmModule)
}

type ModuleSpec struct {
	Path    string
	Args    []string
	Env     []string
	Dirs    []string
	Listens []string
	Dials   []string
	Sockets string
	Stdin   int
	Stdout  int
	Stderr  int
}

func (s ModuleSpec) Copy() (copy ModuleSpec) {
	copy = s
	copy.Args = slices.Clone(s.Args)
	copy.Env = slices.Clone(s.Env)
	copy.Dirs = slices.Clone(s.Dirs)
	copy.Listens = slices.Clone(s.Listens)
	copy.Dials = slices.Clone(s.Dials)
	return
}

type LogSpec struct {
	ID          uuid.UUID
	Compression timemachine.Compression
	BatchSize   int
}

type PreparedModule struct {
	moduleSpec ModuleSpec
	wasmName   string
	wasmCode   []byte
	wasmModule wazero.CompiledModule
	builder    *imports.Builder

	logSpec  *LogSpec
	recorder func(wasi.System) wasi.System

	trace io.Writer

	cleanup []func() error
}

// SetTrace sets the io.Writer that receives a trace of system calls
// when the module is executed.
func (p *PreparedModule) SetTrace(w io.Writer) {
	p.trace = w
}

// Close closes the module.
func (p *PreparedModule) Close(ctx context.Context) error {
	var errs []error
	if len(p.cleanup) > 0 {
		for i := len(p.cleanup) - 1; i >= 0; i-- {
			if err := p.cleanup[i](); err != nil {
				errs = append(errs, err)
			}
		}
	}
	if err := p.wasmModule.Close(ctx); err != nil {
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
