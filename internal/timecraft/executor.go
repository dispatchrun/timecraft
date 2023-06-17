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

	"golang.org/x/sync/errgroup"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/timecraft/sdk"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
)

// Executor executes WebAssembly modules.
type Executor struct {
	registry *timemachine.Registry
	runtime  wazero.Runtime

	group *errgroup.Group
	ctx   context.Context
}

// NewExecutor creates an Executor.
func NewExecutor(ctx context.Context, registry *timemachine.Registry, runtime wazero.Runtime) *Executor {
	r := &Executor{
		registry: registry,
		runtime:  runtime,
	}
	r.group, r.ctx = errgroup.WithContext(ctx)
	return r
}

// Start starts a WebAssembly module.
//
// The ModuleSpec describes the module to be executed. An optional LogSpec
// can be provided to record a trace of execution to a log when the module
// is executed.
//
// If Start returns an error it indicates that there was a problem
// initializing the WebAssembly module. If the WebAssembly module starts
// successfully, any errors that occur during execution must be retrieved
// via Wait.
func (e *Executor) Start(moduleSpec ModuleSpec, logSpec *LogSpec) error {
	wasmPath := moduleSpec.Path
	wasmName := filepath.Base(wasmPath)
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("could not read wasm file '%s': %w", wasmPath, err)
	}

	wasmModule, err := e.runtime.CompileModule(e.ctx, wasmCode)
	if err != nil {
		return err
	}

	wasiBuilder := imports.NewBuilder().
		WithName(wasmName).
		WithArgs(moduleSpec.Args...).
		WithEnv(moduleSpec.Env...).
		WithDirs(moduleSpec.Dirs...).
		WithListens(moduleSpec.Listens...).
		WithDials(moduleSpec.Dials...).
		WithStdio(moduleSpec.Stdin, moduleSpec.Stdout, moduleSpec.Stderr).
		WithSocketsExtension(moduleSpec.Sockets, wasmModule)

	var logSegment io.WriteCloser
	var recordWriter *timemachine.LogRecordWriter
	var recorder func(wasi.System) wasi.System

	if logSpec != nil {
		module, err := e.registry.CreateModule(e.ctx, &format.Module{
			Code: wasmCode,
		}, object.Tag{
			Name:  "timecraft.module.name",
			Value: wasmModule.Name(),
		})
		if err != nil {
			return err
		}

		runtime, err := e.registry.CreateRuntime(e.ctx, &format.Runtime{
			Runtime: "timecraft",
			Version: Version(),
		})
		if err != nil {
			return err
		}

		config, err := e.registry.CreateConfig(e.ctx, &format.Config{
			Runtime: runtime,
			Modules: []*format.Descriptor{module},
			Args:    append([]string{wasmName}, moduleSpec.Args...),
			Env:     moduleSpec.Env,
		})
		if err != nil {
			return err
		}

		process, err := e.registry.CreateProcess(e.ctx, &format.Process{
			ID:        logSpec.ProcessID,
			StartTime: logSpec.StartTime,
			Config:    config,
		})
		if err != nil {
			return err
		}

		if err := e.registry.CreateLogManifest(e.ctx, logSpec.ProcessID, &format.Manifest{
			Process:   process,
			StartTime: logSpec.StartTime,
		}); err != nil {
			return err
		}

		// TODO: create a writer that writes to many segments
		logSegment, err = e.registry.CreateLogSegment(e.ctx, logSpec.ProcessID, 0)
		if err != nil {
			return err
		}
		logWriter := timemachine.NewLogWriter(logSegment)
		recordWriter = timemachine.NewLogRecordWriter(logWriter, logSpec.BatchSize, logSpec.Compression)

		recorder = func(s wasi.System) wasi.System {
			var b timemachine.RecordBuilder
			return wasicall.NewRecorder(s, func(id wasicall.SyscallID, syscallBytes []byte) {
				b.Reset(logSpec.StartTime)
				b.SetTimestamp(time.Now())
				b.SetFunctionID(int(id))
				b.SetFunctionCall(syscallBytes)
				if err := recordWriter.WriteRecord(&b); err != nil {
					panic(err) // TODO: better error handling
				}
			})
		}
	}

	// Setup a gRPC server for the module so that it can interact with the
	// timecraft runtime.
	server := moduleServer{
		executor:   e,
		moduleSpec: moduleSpec,
		logSpec:    logSpec,
	}
	serverSocket := path.Join(os.TempDir(), fmt.Sprintf("timecraft.%s.sock", uuid.NewString()))
	serverListener, err := net.Listen("unix", serverSocket)
	if err != nil {
		return err
	}
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
	if moduleSpec.Trace != nil {
		wrappers = append(wrappers, func(system wasi.System) wasi.System {
			return wasi.Trace(moduleSpec.Trace, system)
		})
	}
	if recorder != nil {
		wrappers = append(wrappers, recorder)
	}
	wasiBuilder = wasiBuilder.WithWrappers(wrappers...)

	ctx, cancel := context.WithCancel(e.ctx)

	// Bring it all together!
	var system wasi.System
	ctx, system, err = wasiBuilder.Instantiate(ctx, e.runtime)
	if err != nil {
		cancel()
		return err
	}

	// Run the module in the background, and tidy up once complete.
	e.group.Go(func() error {
		defer cancel()
		defer os.Remove(serverSocket)
		defer serverListener.Close()
		defer system.Close(ctx)
		defer wasmModule.Close(ctx)
		defer func() {
			if logSpec != nil {
				_ = recordWriter.Flush()
				_ = logSegment.Close()
			}
		}()

		return runModule(ctx, e.runtime, wasmModule)
	})

	return nil
}

// Wait blocks until all WebAssembly modules have finished executing.
func (e *Executor) Wait() error {
	return e.group.Wait()
}
