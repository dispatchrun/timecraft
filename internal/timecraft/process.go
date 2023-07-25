package timecraft

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"
	"sync"

	"github.com/google/uuid"
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

type Engine interface {
	Start(moduleSpec ModuleSpec, logSpec *LogSpec, parentID *ProcessID) (ProcessID, error)
	Lookup(processID ProcessID) (process ProcessInfo, ok bool)
	Wait(processID ProcessID) error
	WaitAll() error // TODO: pass context to WaitAll?
	// TODO: missing kill/stop/close?
}

type ProcessManager struct {
	engines map[string]Engine

	ctx    context.Context
	cancel context.CancelCauseFunc

	processes map[ProcessID]*ProcessInfo
	mu        sync.Mutex
}

func NewProcessManager(ctx context.Context, engines map[string]Engine) *ProcessManager {
	ctx, cancel := context.WithCancelCause(ctx)

	pm := &ProcessManager{
		ctx:     ctx,
		cancel:  cancel,
		engines: engines,
	}

	return pm
}

// Start a process following the given specification
func (pm *ProcessManager) Start(moduleSpec ModuleSpec, logSpec *LogSpec, parentID *ProcessID) (ProcessID, error) {
	eng, ok := pm.engines[moduleSpec.Engine]
	if !ok {
		return ProcessID{}, fmt.Errorf("unknown engine %q", moduleSpec.Engine)
	}
	return eng.Start(moduleSpec, logSpec, parentID)
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
	for _, eng := range pm.engines {
		if _, ok := eng.Lookup(processID); ok {
			return eng.Wait(processID)
		}
	}
	return fmt.Errorf("unknown process %s", processID)
}

// Wait for all processes to exit.
func (pm *ProcessManager) WaitAll() error {
	for _, eng := range pm.engines {
		if err := eng.WaitAll(); err != nil {
			return err
		}
	}
	return nil
}

// Close the process manager.
func (pm *ProcessManager) Close() error {
	pm.cancel(nil)
	return pm.WaitAll()
}
