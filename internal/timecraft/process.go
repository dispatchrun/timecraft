package timecraft

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"sync"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero"
)

// ProcessID is a process identifier.
type ProcessID = uuid.UUID

// ProcessInfo is information about a process.
type ProcessInfo struct {
	// ID is the ID of the process.
	ID ProcessID

	// Addr is a unique IP address for the process.
	Addr netip.Addr

	// Transport is an HTTP transport that can be used to send work to
	// the process over the work socket.
	Transport *http.Transport

	// ParentID is the ID of the process that spawned this one (if applicable).
	ParentID *ProcessID

	// ModuleSpec is the module specification of the process.
	ModuleSpec ModuleSpec

	// ContainerID used by the runsc engine to retrieve the process.
	ContainerID string

	ctx    context.Context
	cancel context.CancelCauseFunc
}

type ProcessManager struct {
	runsc *RunscEngine
	wasm  *WasmEngine

	ctx    context.Context
	cancel context.CancelCauseFunc

	processes map[ProcessID]*ProcessInfo
	mu        sync.Mutex
}

func NewProcessManager(ctx context.Context, registry *timemachine.Registry, runtime wazero.Runtime, serverFactory *ServerFactory, adapter func(ProcessID, wasi.System) wasi.System) *ProcessManager {
	ctx, cancel := context.WithCancelCause(ctx)

	pm := &ProcessManager{
		ctx:    ctx,
		cancel: cancel,
		runsc:  NewRunscEngine(ctx),
		wasm:   NewWasmEngine(ctx, registry, runtime, serverFactory, adapter),
	}

	return pm
}

// Start a process following the given specification
func (pm *ProcessManager) Start(moduleSpec ModuleSpec, logSpec *LogSpec, parentID *ProcessID) (ProcessID, error) {
	switch moduleSpec.Engine {
	case "runsc":
		return pm.runsc.Start(moduleSpec, logSpec, parentID)
	case "wasm":
		return pm.wasm.Start(moduleSpec, logSpec, parentID)
	}
	return ProcessID{}, fmt.Errorf("unknown engine %q", moduleSpec.Engine)
}

// Lookup a process by ID.
func (pm *ProcessManager) Lookup(processID ProcessID) (process ProcessInfo, ok bool) {
	pm.mu.Lock()
	var p *ProcessInfo
	if p, ok = pm.processes[processID]; ok {
		process = *p // copy
	}
	pm.mu.Unlock()
	return
}

// Wait for a process to exit.
func (pm *ProcessManager) Wait(processID ProcessID) error {
	process, ok := pm.Lookup(processID)
	if !ok {
		return fmt.Errorf("process %v not found", processID)
	}
	switch process.ModuleSpec.Engine {
	case "runsc":
		return pm.runsc.Wait(processID)
	case "wasm":
		return pm.wasm.Wait(processID)
	}
	return nil
}

// Wait for all processes to exit.
func (pm *ProcessManager) WaitAll() error {
	if err := pm.wasm.WaitAll(); err != nil {
		return err
	}
	return pm.runsc.WaitAll()
}

// Close the process manager.
func (pm *ProcessManager) Close() error {
	pm.cancel(nil)
	return pm.WaitAll()
}
