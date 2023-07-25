package timecraft

import (
	"context"
	"fmt"
	"math/rand"
	"sync"

	"github.com/containerd/go-runc"
	"github.com/google/uuid"
	shim "gvisor.dev/gvisor/pkg/shim/runsc"
)

type RunscEngine struct {
	processes map[ProcessID]*ProcessInfo
	mu        sync.Mutex

	ctx    context.Context
	cancel context.CancelCauseFunc

	runsc *shim.Runsc
}

const (
	runscBin = "runsc"
	// TODO: hook with timecraft.Registry
	runscRoot = "/var/run/timecraft/runsc"
)

// TODO: garbage collect old containers
func NewRunscEngine(ctx context.Context) *RunscEngine {
	return &RunscEngine{
		ctx: ctx,
		runsc: &shim.Runsc{
			Command: runscBin,
			Root:    runscRoot,
		},
	}
}

// Start a process following the given specification
func (e *RunscEngine) Start(moduleSpec ModuleSpec, logSpec *LogSpec, parentID *ProcessID) (ProcessID, error) {
	cid := fmt.Sprintf("timecraft-%06d", rand.Int31n(1000000))

	if err := e.runsc.Create(e.ctx, cid, moduleSpec.BundleDir, nil); err != nil {
		return ProcessID{}, err
	}

	processID := uuid.New()

	//FIXME
	var cio runc.IO

	// TODO: cleanup created container on start failure
	if err := e.runsc.Start(e.ctx, cid, cio); err != nil {
		return processID, err
	}

	return ProcessID{}, nil
}

// Lookup a process by ID.
func (e *RunscEngine) Lookup(processID ProcessID) (process ProcessInfo, ok bool) {
	e.mu.Lock()
	var p *ProcessInfo
	if p, ok = e.processes[processID]; ok {
		process = *p // copy
	}
	e.mu.Unlock()
	return
}

// Wait for a process to exit.
func (e *RunscEngine) Wait(processID ProcessID) error {
	e.mu.Lock()
	p, ok := e.processes[processID]
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("process %v not found", processID)
	}

	_, err := e.runsc.Wait(e.ctx, p.ContainerID)
	return err
}

// Wait for all processes to exit.
func (e *RunscEngine) WaitAll() error {
	panic("not implemented") // TODO: Implement
}

// Close the process manager.
func (e *RunscEngine) Close() error {
	return nil
}
