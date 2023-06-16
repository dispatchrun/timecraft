package timecraft

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
)

// ModuleSpec is the details about what WebAssembly module to execute,
// how it should be initialized, and what its inputs are.
type ModuleSpec struct {
	// Path is the path of the WebAssembly module.
	Path string

	// Args are command-line arguments to pass to the module.
	Args []string

	// Env is the environment variables to pass to the module.
	Env []string

	// Dirs is a set of directories to make available to the module.
	Dirs []string

	// Listens is a set of listener sockets to make available to the module.
	Listens []string

	// Dials is a set of connection sockets to make available to the module.
	Dials []string

	// Sockets is the name of a sockets extension to use, or "auto" to
	// automatically detect the sockets extension.
	Sockets string

	// Stdio file descriptors.
	Stdin  int
	Stdout int
	Stderr int

	// Trace is an optional writer that receives a trace of system calls
	// made by the module.
	Trace io.Writer
}

// LogSpec is details about the log that records a trace of execution.
type LogSpec struct {
	ProcessID   uuid.UUID
	StartTime   time.Time
	Compression timemachine.Compression
	BatchSize   int
}

// Module is a WebAssembly module.
type Module struct {
	moduleSpec  ModuleSpec
	wasmModule  wazero.CompiledModule
	wasiBuilder *imports.Builder

	logSpec      *LogSpec
	logSegment   io.WriteCloser
	recordWriter *timemachine.LogRecordWriter
	recorder     func(wasi.System) wasi.System

	run    bool
	ctx    context.Context
	cancel context.CancelFunc
}

// Close closes the module.
func (m *Module) Close() error {
	m.cancel()

	if m.logSpec != nil {
		// TODO: handle errors
		_ = m.recordWriter.Flush()
		_ = m.logSegment.Close()
	}
	return m.wasmModule.Close(m.ctx)
}
