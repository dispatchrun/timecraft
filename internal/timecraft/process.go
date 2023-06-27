package timecraft

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/timecraft/sdk"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
)

// ProcessManager runs WebAssembly modules.
//
// A running WebAssembly module is known as a process. Processes are allowed
// to spawn other processes. The ProcessManager manages the lifecycle of
// processes.
type ProcessManager struct {
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
	// ID is the ID of the process.
	ID ProcessID

	// Transport is an HTTP transport that can be used to send work to
	// the process over the work socket.
	Transport *http.Transport

	ctx    context.Context
	cancel context.CancelCauseFunc
}

// NewProcessManager creates an ProcessManager.
func NewProcessManager(ctx context.Context, registry *timemachine.Registry, runtime wazero.Runtime, serverFactory *ServerFactory) *ProcessManager {
	r := &ProcessManager{
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
// can be provided to instruct the ProcessManager to record a trace of execution
// to a log.
//
// If Start returns an error it indicates that there was a problem
// initializing the WebAssembly module. If the WebAssembly module starts
// successfully, any errors that occur during execution must be retrieved
// via Wait or WaitAll.
func (pm *ProcessManager) Start(moduleSpec ModuleSpec, logSpec *LogSpec) (ProcessID, error) {
	wasmPath := moduleSpec.Path
	wasmName := filepath.Base(wasmPath)
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return ProcessID{}, fmt.Errorf("could not read wasm file '%s': %w", wasmPath, err)
	}

	wasmModule, err := pm.runtime.CompileModule(pm.ctx, wasmCode)
	if err != nil {
		return ProcessID{}, err
	}

	dialer := &net.Dialer{}
	listen := &net.ListenConfig{}

	options := []sandbox.Option{
		sandbox.Args(append([]string{wasmName}, moduleSpec.Args...)...),
		sandbox.Environ(moduleSpec.Env...),
		sandbox.Time(time.Now),
		sandbox.Rand(rand.Reader),
		sandbox.Dial(dialer.DialContext),
		sandbox.Listen(listen.Listen),
		sandbox.ListenPacket(listen.ListenPacket),
		sandbox.NameResolver(net.DefaultResolver),
	}

	for _, dir := range moduleSpec.Dirs {
		options = append(options, sandbox.Mount(dir, sandbox.DirFS(dir)))
	}

	guest := sandbox.New(options...)

	for _, addr := range moduleSpec.Listens {
		if err := listenTCP(pm.ctx, guest, addr); err != nil {
			return ProcessID{}, err
		}
	}

	for _, addr := range moduleSpec.Dials {
		if err := dialTCP(pm.ctx, guest, addr); err != nil {
			return ProcessID{}, err
		}
	}

	var system wasi.System = guest
	var logSegment io.WriteCloser
	var recordWriter *timemachine.LogRecordWriter
	var processID ProcessID
	if logSpec != nil && logSpec.ProcessID != (ProcessID{}) {
		processID = logSpec.ProcessID
	} else {
		processID = uuid.New()
	}

	if logSpec != nil {
		logSpec.ProcessID = processID

		module, err := pm.registry.CreateModule(pm.ctx, &format.Module{
			Code: wasmCode,
		}, object.Tag{
			Name:  "timecraft.module.name",
			Value: wasmModule.Name(),
		})
		if err != nil {
			return ProcessID{}, err
		}

		runtime, err := pm.registry.CreateRuntime(pm.ctx, &format.Runtime{
			Runtime: "timecraft",
			Version: Version(),
		})
		if err != nil {
			return ProcessID{}, err
		}

		config, err := pm.registry.CreateConfig(pm.ctx, &format.Config{
			Runtime: runtime,
			Modules: []*format.Descriptor{module},
			Args:    append([]string{wasmName}, moduleSpec.Args...),
			Env:     moduleSpec.Env,
		})
		if err != nil {
			return ProcessID{}, err
		}

		process, err := pm.registry.CreateProcess(pm.ctx, &format.Process{
			ID:        logSpec.ProcessID,
			StartTime: logSpec.StartTime,
			Config:    config,
		})
		if err != nil {
			return ProcessID{}, err
		}

		if err := pm.registry.CreateLogManifest(pm.ctx, logSpec.ProcessID, &format.Manifest{
			Process:   process,
			StartTime: logSpec.StartTime,
		}); err != nil {
			return ProcessID{}, err
		}

		// TODO: create a writer that writes to many segments
		logSegment, err = pm.registry.CreateLogSegment(pm.ctx, logSpec.ProcessID, 0)
		if err != nil {
			return ProcessID{}, err
		}
		logWriter := timemachine.NewLogWriter(logSegment)
		recordWriter = timemachine.NewLogRecordWriter(logWriter, logSpec.BatchSize, logSpec.Compression)

		var b timemachine.RecordBuilder
		system = wasicall.NewRecorder(system, func(id wasicall.SyscallID, syscallBytes []byte) {
			b.Reset(logSpec.StartTime)
			b.SetTimestamp(time.Now())
			b.SetFunctionID(int(id))
			b.SetFunctionCall(syscallBytes)
			if err := recordWriter.WriteRecord(&b); err != nil {
				panic(err) // caught/handled by wazero
			}
		})
	} else {
		processID = uuid.New()
	}

	// Setup a gRPC server for the module so that it can interact with the
	// timecraft runtime.
	server := pm.serverFactory.NewServer(pm.ctx, processID, moduleSpec, logSpec)
	serverListener, err := guest.Listen(pm.ctx, "tcp", sdk.TimecraftAddress)
	if err != nil {
		return ProcessID{}, err
	}
	go func() {
		if err := server.Serve(serverListener); err != nil && !errors.Is(err, net.ErrClosed) {
			pm.cancel(fmt.Errorf("failed to serve gRPC server: %w", err))
		}
	}()

	if moduleSpec.Trace != nil {
		system = wasi.Trace(moduleSpec.Trace, system)
	}

	if moduleSpec.Stdin != nil {
		stdin := guest.Stdin()
		go func() { _, _ = io.Copy(stdin, moduleSpec.Stdin); stdin.Close() }()
	}
	if moduleSpec.Stdout == nil {
		moduleSpec.Stdout = io.Discard
	}
	if moduleSpec.Stderr == nil {
		moduleSpec.Stderr = io.Discard
	}
	stdout := guest.Stdout()
	stderr := guest.Stderr()
	go func() { _, _ = io.Copy(moduleSpec.Stdout, stdout); stdout.Close() }()
	go func() { _, _ = io.Copy(moduleSpec.Stderr, stderr); stderr.Close() }()

	extensions := imports.DetectExtensions(wasmModule)
	hostModule := wasi_snapshot_preview1.NewHostModule(extensions...)
	wasiModule := wazergo.MustInstantiate(pm.ctx, pm.runtime,
		hostModule,
		wasi_snapshot_preview1.WithWASI(system),
	)
	if err != nil {
		return ProcessID{}, err
	}

	ctx := wazergo.WithModuleInstance(pm.ctx, wasiModule)
	ctx, cancel := context.WithCancelCause(ctx)

	process := &ProcessInfo{
		ID: processID,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
				// The process isn't necessarily available to take on work immediately.
				// Retry with exponential backoff when an ECONNREFUSED is encountered.
				// TODO: make these configurable?
				const (
					maxAttempts = 10
					minDelay    = 500 * time.Millisecond
					maxDelay    = 5 * time.Second
				)
				retry(ctx, maxAttempts, minDelay, maxDelay, func() bool {
					conn, err = guest.Dial(ctx, "tcp", sdk.WorkAddress)
					switch {
					case errors.Is(err, syscall.ECONNREFUSED):
						return true
					case errors.Is(err, syscall.ENOENT):
						return true
					default:
						return false
					}
				})
				return
			},
		},
		ctx:    ctx,
		cancel: cancel,
	}

	pm.mu.Lock()
	pm.processes[processID] = process
	pm.mu.Unlock()

	// Run the module in the background, and tidy up once complete.
	pm.group.Go(func() (err error) {
		defer serverListener.Close()
		defer server.Close()
		defer system.Close(ctx)
		defer wasiModule.Close(ctx)
		defer wasmModule.Close(ctx)
		if logSpec != nil {
			defer logSegment.Close()
			defer recordWriter.Flush()
		}
		defer func() {
			pm.mu.Lock()
			delete(pm.processes, processID)
			pm.mu.Unlock()
		}()
		defer func() { cancel(err) }()
		err = runModule(ctx, pm.runtime, wasmModule)
		return
	})

	return processID, nil
}

// Lookup looks up a process by ID.
//
// The return flag is true if the process exists and is alive, and
// false otherwise.
func (pm *ProcessManager) Lookup(processID ProcessID) (process ProcessInfo, ok bool) {
	pm.mu.Lock()
	var p *ProcessInfo
	if p, ok = pm.processes[processID]; ok {
		process = *p // copy
	}
	pm.mu.Unlock()
	return
}

// Wait blocks until a process exits.
func (pm *ProcessManager) Wait(processID ProcessID) error {
	pm.mu.Lock()
	p, ok := pm.processes[processID]
	pm.mu.Unlock()

	if !ok {
		return errors.New("process not found")
	}

	<-p.ctx.Done()

	err := context.Cause(p.ctx)
	switch err {
	case context.Canceled:
		err = nil
	}
	return err
}

// WaitAll blocks until all processes have exited.
func (pm *ProcessManager) WaitAll() error {
	return pm.group.Wait()
}

// Close closes the process manager.
func (pm *ProcessManager) Close() error {
	pm.cancel(nil)
	return pm.WaitAll()
}

func dialTCP(ctx context.Context, system *sandbox.System, address string) error {
	addrInfos, err := getaddrinfo(ctx, system, address, wasi.AddressInfo{
		Family:     wasi.UnspecifiedFamily,
		SocketType: wasi.StreamSocket,
		Protocol:   wasi.TCPProtocol,
	})
	if err != nil {
		return err
	}

	var lastSyscall string
	var lastErrno wasi.Errno

	for _, addrInfo := range addrInfos {
		socket, errno := system.SockOpen(ctx, addrInfo.Family, addrInfo.SocketType, addrInfo.Protocol,
			wasi.SockConnectionRights,
			wasi.SockConnectionRights,
		)
		if errno != wasi.ESUCCESS {
			lastSyscall, lastErrno = "socket", errno
			continue
		}
		if _, errno := system.SockConnect(ctx, socket, addrInfo.Address); errno != wasi.ESUCCESS {
			_ = system.FDClose(ctx, socket)
			lastSyscall, lastErrno = "connect", errno
			continue
		}
		_ = system.FDStatSetFlags(ctx, socket, wasi.NonBlock)
		system.PreopenFD(socket)
		return nil
	}

	return os.NewSyscallError(lastSyscall, lastErrno.Syscall())
}

func listenTCP(ctx context.Context, system *sandbox.System, address string) error {
	addrInfos, err := getaddrinfo(ctx, system, address, wasi.AddressInfo{
		Flags:      wasi.Passive,
		Family:     wasi.UnspecifiedFamily,
		SocketType: wasi.StreamSocket,
		Protocol:   wasi.TCPProtocol,
	})
	if err != nil {
		return err
	}

	var lastSyscall string
	var lastErrno wasi.Errno

	for _, addrInfo := range addrInfos {
		socket, errno := system.SockOpen(ctx, addrInfo.Family, addrInfo.SocketType, addrInfo.Protocol,
			wasi.SockListenRights,
			wasi.SockConnectionRights,
		)
		if errno != wasi.ESUCCESS {
			lastSyscall, lastErrno = "socket", errno
			continue
		}
		if _, errno := system.SockBind(ctx, socket, addrInfo.Address); errno != wasi.ESUCCESS {
			_ = system.FDClose(ctx, socket)
			lastSyscall, lastErrno = "bind", errno
			continue
		}
		if errno := system.SockListen(ctx, socket, 0); errno != wasi.ESUCCESS {
			_ = system.FDClose(ctx, socket)
			lastSyscall, lastErrno = "listen", errno
			continue
		}
		_ = system.FDStatSetFlags(ctx, socket, wasi.NonBlock)
		system.PreopenFD(socket)
		return nil
	}

	return os.NewSyscallError(lastSyscall, lastErrno.Syscall())
}

func getaddrinfo(ctx context.Context, system wasi.System, address string, hints wasi.AddressInfo) ([]wasi.AddressInfo, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	results := make([]wasi.AddressInfo, 8)
	n, errno := system.SockAddressInfo(ctx, host, port, hints, results)
	if errno != wasi.ESUCCESS {
		return nil, os.NewSyscallError("getaddrinfo", errno.Syscall())
	}
	return results[:n], nil
}
