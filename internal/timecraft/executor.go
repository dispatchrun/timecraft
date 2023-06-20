package timecraft

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/stealthrocket/timecraft/sdk"
	"golang.org/x/sync/errgroup"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
)

// Executor executes WebAssembly modules.
//
// A running WebAssembly module is known as a process. Processes are allowed
// to spawn other processes. The Executor thus manages the lifecycle of
// processes. Two methods are provided: Start to start a process, and Wait to
// block until all processes have finished executing.
type Executor struct {
	registry      *timemachine.Registry
	runtime       wazero.Runtime
	serverFactory *ServerFactory

	processes map[ProcessID]*ProcessInfo
	mu        sync.Mutex

	group  *errgroup.Group
	ctx    context.Context
	cancel context.CancelCauseFunc
}

// ProcessID is a process identifier.
type ProcessID = uuid.UUID

// ProcessInfo is information about a process.
type ProcessInfo struct {
	ID         ProcessID
	WorkSocket string

	cancel context.CancelFunc
}

// NewExecutor creates an Executor.
func NewExecutor(ctx context.Context, registry *timemachine.Registry, runtime wazero.Runtime, serverFactory *ServerFactory) *Executor {
	r := &Executor{
		registry:      registry,
		runtime:       runtime,
		serverFactory: serverFactory,
		processes:     map[ProcessID]*ProcessInfo{},
	}
	r.group, ctx = errgroup.WithContext(ctx)
	r.ctx, r.cancel = context.WithCancelCause(ctx)
	return r
}

// Start starts a process.
//
// The ModuleSpec describes the module to be executed. An optional LogSpec
// can be provided to instruct the Executor to record a trace of execution
// to a log.
//
// If Start returns an error it indicates that there was a problem
// initializing the WebAssembly module. If the WebAssembly module starts
// successfully, any errors that occur during execution must be retrieved
// via Wait.
func (e *Executor) Start(moduleSpec ModuleSpec, logSpec *LogSpec) (ProcessID, error) {
	wasmPath := moduleSpec.Path
	wasmName := filepath.Base(wasmPath)
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return ProcessID{}, fmt.Errorf("could not read wasm file '%s': %w", wasmPath, err)
	}

	wasmModule, err := e.runtime.CompileModule(e.ctx, wasmCode)
	if err != nil {
		return ProcessID{}, err
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

	var processID ProcessID
	if logSpec != nil && logSpec.ProcessID != (ProcessID{}) {
		processID = logSpec.ProcessID
	} else {
		processID = uuid.New()
	}

	if logSpec != nil {
		logSpec.ProcessID = processID

		module, err := e.registry.CreateModule(e.ctx, &format.Module{
			Code: wasmCode,
		}, object.Tag{
			Name:  "timecraft.module.name",
			Value: wasmModule.Name(),
		})
		if err != nil {
			return ProcessID{}, err
		}

		runtime, err := e.registry.CreateRuntime(e.ctx, &format.Runtime{
			Runtime: "timecraft",
			Version: Version(),
		})
		if err != nil {
			return ProcessID{}, err
		}

		config, err := e.registry.CreateConfig(e.ctx, &format.Config{
			Runtime: runtime,
			Modules: []*format.Descriptor{module},
			Args:    append([]string{wasmName}, moduleSpec.Args...),
			Env:     moduleSpec.Env,
		})
		if err != nil {
			return ProcessID{}, err
		}

		process, err := e.registry.CreateProcess(e.ctx, &format.Process{
			ID:        logSpec.ProcessID,
			StartTime: logSpec.StartTime,
			Config:    config,
		})
		if err != nil {
			return ProcessID{}, err
		}

		if err := e.registry.CreateLogManifest(e.ctx, logSpec.ProcessID, &format.Manifest{
			Process:   process,
			StartTime: logSpec.StartTime,
		}); err != nil {
			return ProcessID{}, err
		}

		// TODO: create a writer that writes to many segments
		logSegment, err = e.registry.CreateLogSegment(e.ctx, logSpec.ProcessID, 0)
		if err != nil {
			return ProcessID{}, err
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
					panic(err) // caught/handled by wazero
				}
			})
		}
	} else {
		processID = uuid.New()
	}

	// Setup a gRPC server for the module so that it can interact with the
	// timecraft runtime.
	server := e.serverFactory.NewServer(processID, moduleSpec, logSpec)
	timecraftSocket, timecraftSocketCleanup := makeSocketPath()
	serverListener, err := net.Listen("unix", timecraftSocket)
	if err != nil {
		return ProcessID{}, err
	}
	go func() {
		if err := server.Serve(serverListener); err != nil && !errors.Is(err, net.ErrClosed) {
			e.cancel(fmt.Errorf("failed to serve gRPC server: %w", err))
		}
	}()

	workSocket, workSocketCleanup := makeSocketPath()

	// Wrap the wasi.System to make the gRPC server accessible via a virtual
	// socket, to trace system calls (if applicable), and to record system
	// calls to the log. The order is important here!
	var wrappers []func(wasi.System) wasi.System
	wrappers = append(wrappers, func(system wasi.System) wasi.System {
		return NewVirtualSocketsSystem(system, map[string]string{
			sdk.TimecraftSocket: timecraftSocket,
			sdk.WorkSocket:      workSocket,
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
	ctx, system, err := wasiBuilder.WithWrappers(wrappers...).Instantiate(e.ctx, e.runtime)
	if err != nil {
		return ProcessID{}, err
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	e.mu.Lock()
	e.processes[processID] = &ProcessInfo{
		ID:         processID,
		WorkSocket: workSocket,
		cancel:     cancel,
	}
	e.mu.Unlock()

	// Run the module in the background, and tidy up once complete.
	e.group.Go(func() error {
		defer timecraftSocketCleanup()
		defer workSocketCleanup()
		defer serverListener.Close()
		defer system.Close(ctx)
		defer wasmModule.Close(ctx)
		if logSpec != nil {
			defer logSegment.Close()
			defer recordWriter.Flush()
		}
		defer func() {
			e.mu.Lock()
			delete(e.processes, processID)
			e.mu.Unlock()
		}()

		return runModule(ctx, e.runtime, wasmModule)
	})

	return processID, nil
}

// Lookup looks up a process by ID.
//
// The return flag is true if the process exists and is alive, and
// false otherwise.
func (e *Executor) Lookup(processID ProcessID) (info *ProcessInfo, ok bool) {
	e.mu.Lock()
	info, ok = e.processes[processID]
	e.mu.Unlock()
	return
}

// Wait blocks until all processes have finished executing.
func (e *Executor) Wait() error {
	return e.group.Wait()
}
