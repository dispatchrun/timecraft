package timecraft

import (
	"context"
	"errors"
	"fmt"
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
//
// The ModuleSpec describes the module to be executed. An optional LogSpec
// can be provided to record a trace of execution to a log when the Module
// is executed.
func (r *Runner) Prepare(moduleSpec ModuleSpec, logSpec *LogSpec) (*Module, error) {
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

	m := &Module{
		ctx:         ctx,
		cancel:      cancel,
		moduleSpec:  moduleSpec,
		wasmModule:  wasmModule,
		wasiBuilder: builder,
	}

	if logSpec != nil {
		m.logSpec = logSpec

		module, err := r.registry.CreateModule(r.ctx, &format.Module{
			Code: wasmCode,
		}, object.Tag{
			Name:  "timecraft.module.name",
			Value: wasmModule.Name(),
		})
		if err != nil {
			return nil, err
		}

		runtime, err := r.registry.CreateRuntime(r.ctx, &format.Runtime{
			Runtime: "timecraft",
			Version: Version(),
		})
		if err != nil {
			return nil, err
		}

		config, err := r.registry.CreateConfig(r.ctx, &format.Config{
			Runtime: runtime,
			Modules: []*format.Descriptor{module},
			Args:    append([]string{wasmName}, moduleSpec.Args...),
			Env:     moduleSpec.Env,
		})
		if err != nil {
			return nil, err
		}

		process, err := r.registry.CreateProcess(r.ctx, &format.Process{
			ID:        logSpec.ProcessID,
			StartTime: logSpec.StartTime,
			Config:    config,
		})
		if err != nil {
			return nil, err
		}

		if err := r.registry.CreateLogManifest(r.ctx, logSpec.ProcessID, &format.Manifest{
			Process:   process,
			StartTime: logSpec.StartTime,
		}); err != nil {
			return nil, err
		}

		// TODO: create a writer that writes to many segments
		m.logSegment, err = r.registry.CreateLogSegment(r.ctx, logSpec.ProcessID, 0)
		if err != nil {
			return nil, err
		}
		logWriter := timemachine.NewLogWriter(m.logSegment)
		m.recordWriter = timemachine.NewLogRecordWriter(logWriter, logSpec.BatchSize, logSpec.Compression)

		m.recorder = func(s wasi.System) wasi.System {
			var b timemachine.RecordBuilder
			return wasicall.NewRecorder(s, func(id wasicall.SyscallID, syscallBytes []byte) {
				b.Reset(logSpec.StartTime)
				b.SetTimestamp(time.Now())
				b.SetFunctionID(int(id))
				b.SetFunctionCall(syscallBytes)
				if err := m.recordWriter.WriteRecord(&b); err != nil {
					panic(err) // TODO: better error handling
				}
			})
		}
	}

	return m, nil
}

// Run runs a prepared WebAssembly module.
func (r *Runner) Run(m *Module) error {
	if m.run {
		return errors.New("module is already running or has already run")
	}
	m.run = true

	server := Server{
		Runner:  r,
		Module:  m.moduleSpec,
		Log:     m.logSpec,
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
	if m.moduleSpec.Trace != nil {
		wrappers = append(wrappers, func(system wasi.System) wasi.System {
			return wasi.Trace(m.moduleSpec.Trace, system)
		})
	}
	if m.recorder != nil {
		wrappers = append(wrappers, m.recorder)
	}
	m.wasiBuilder = m.wasiBuilder.WithWrappers(wrappers...)

	var system wasi.System
	m.ctx, system, err = m.wasiBuilder.Instantiate(m.ctx, r.runtime)
	if err != nil {
		return err
	}
	defer system.Close(m.ctx)

	return runModule(m.ctx, r.runtime, m.wasmModule)
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
