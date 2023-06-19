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

	"golang.org/x/sync/errgroup"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/httproxy"
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

	processes map[uuid.UUID]*process
	mu        sync.Mutex

	group  *errgroup.Group
	ctx    context.Context
	cancel context.CancelCauseFunc
}

type process struct {
	id      uuid.UUID
	parent  *uuid.UUID
	cancel  context.CancelFunc
	mailbox chan<- message
}

type message struct {
	sender uuid.UUID
	body   []byte
}

// NewExecutor creates an Executor.
func NewExecutor(ctx context.Context, registry *timemachine.Registry, runtime wazero.Runtime) *Executor {
	r := &Executor{
		registry:  registry,
		runtime:   runtime,
		processes: map[uuid.UUID]*process{},
	}
	r.group, ctx = errgroup.WithContext(ctx)
	r.ctx, r.cancel = context.WithCancelCause(ctx)
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
func (e *Executor) Start(moduleSpec ModuleSpec, logSpec *LogSpec, parentID *uuid.UUID) (uuid.UUID, error) {
	wasmPath := moduleSpec.Path
	wasmName := filepath.Base(wasmPath)
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return uuid.UUID{}, fmt.Errorf("could not read wasm file '%s': %w", wasmPath, err)
	}

	wasmModule, err := e.runtime.CompileModule(e.ctx, wasmCode)
	if err != nil {
		return uuid.UUID{}, err
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

	var processID uuid.UUID
	if logSpec != nil && logSpec.ProcessID != (uuid.UUID{}) {
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
			return uuid.UUID{}, err
		}

		runtime, err := e.registry.CreateRuntime(e.ctx, &format.Runtime{
			Runtime: "timecraft",
			Version: Version(),
		})
		if err != nil {
			return uuid.UUID{}, err
		}

		config, err := e.registry.CreateConfig(e.ctx, &format.Config{
			Runtime: runtime,
			Modules: []*format.Descriptor{module},
			Args:    append([]string{wasmName}, moduleSpec.Args...),
			Env:     moduleSpec.Env,
		})
		if err != nil {
			return uuid.UUID{}, err
		}

		process, err := e.registry.CreateProcess(e.ctx, &format.Process{
			ID:        logSpec.ProcessID,
			StartTime: logSpec.StartTime,
			Config:    config,
		})
		if err != nil {
			return uuid.UUID{}, err
		}

		if err := e.registry.CreateLogManifest(e.ctx, logSpec.ProcessID, &format.Manifest{
			Process:   process,
			StartTime: logSpec.StartTime,
		}); err != nil {
			return uuid.UUID{}, err
		}

		// TODO: create a writer that writes to many segments
		logSegment, err = e.registry.CreateLogSegment(e.ctx, logSpec.ProcessID, 0)
		if err != nil {
			return uuid.UUID{}, err
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

	mailbox := make(chan message)

	// Setup a gRPC server for the module so that it can interact with the
	// timecraft runtime.
	server := moduleServer{
		executor:   e,
		processID:  processID,
		parentID:   parentID,
		moduleSpec: moduleSpec,
		logSpec:    logSpec,
		mailbox:    mailbox,
	}
	serverSocket, serverSocketCleanup := makeSocketPath()
	serverListener, err := net.Listen("unix", serverSocket)
	if err != nil {
		return uuid.UUID{}, err
	}
	go func() {
		if err := server.Serve(serverListener); err != nil && !errors.Is(err, net.ErrClosed) {
			e.cancel(fmt.Errorf("failed to serve gRPC server: %w", err))
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
	ctx, system, err := wasiBuilder.WithWrappers(wrappers...).Instantiate(e.ctx, e.runtime)
	if err != nil {
		return uuid.UUID{}, err
	}

	var cancel context.CancelFunc
	ctx, cancel = context.WithCancel(ctx)

	httproxy.Start(ctx)

	e.mu.Lock()
	e.processes[processID] = &process{
		id:      processID,
		parent:  parentID,
		cancel:  cancel,
		mailbox: mailbox,
	}
	e.mu.Unlock()

	// Run the module in the background, and tidy up once complete.
	e.group.Go(func() error {
		defer serverSocketCleanup()
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

var errNotFound = errors.New("process not found")
var errForbidden = errors.New("process is not allowed to perform that operation")

// Send sends a message to a WebAssembly module.
func (e *Executor) Send(processID uuid.UUID, senderID uuid.UUID, msg []byte) error {
	e.mu.Lock()
	p, ok := e.processes[processID]
	if !ok {
		e.mu.Unlock()
		return errNotFound
	}
	mailbox := p.mailbox
	e.mu.Unlock()

	mailbox <- message{sender: senderID, body: msg}
	return nil
}

// Stop stops a WebAssembly module from running.
func (e *Executor) Stop(processID uuid.UUID, parentID *uuid.UUID) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	p, ok := e.processes[processID]
	if !ok {
		return errNotFound
	} else if parentID != nil && (p.parent == nil || *p.parent != *parentID) {
		return errForbidden
	}

	p.cancel()
	return nil
}

// Wait blocks until all WebAssembly modules have finished executing.
func (e *Executor) Wait() error {
	return e.group.Wait()
}
