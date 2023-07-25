package timecraft

import "context"

type Engine interface {
	// Create a process but don't start it
	Create(ctx context.Context, spec ProcessSpec, logSpec *LogSpec) (*ProcessInfo, error)

	// Run a pre-created process
	Run(ctx context.Context, pid ProcessID) error

	// Wait for a process to exit
	Wait(pid ProcessID) error

	// Close a process:
	Close(pid ProcessID) error
}
