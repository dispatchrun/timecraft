package timecraft

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/stealthrocket/timecraft/internal/timemachine"
)

// ModuleSpec is the details about what WebAssembly module to execute,
// how it should be initialized, and what its inputs are.
type ModuleSpec struct {
	// Path is the path of the WebAssembly module.
	Path string

	// Name of the exported function to call in the WebAssembly module.
	// If empty, the _start function will be executed.
	Function string

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
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	// Trace is an optional writer that receives a trace of system calls
	// made by the module.
	Trace io.Writer

	// Allow the module to bind to the host network when opening listening
	// sockets.
	HostNetworkBinding bool
}

// Key is a string that uniquely identifies the ModuleSpec.
func (m *ModuleSpec) Key() string {
	// TODO: we need a hashable key, but ModuleSpec has nested slices. We also
	//  need a string key for the singleflight library. This isn't very
	//  efficient, doesn't consider all fields, and doesn't handle fields that
	//  need canonicalization (e.g. sorting of environment variables)

	var b strings.Builder
	b.WriteString(fmt.Sprintf(":%d:%s", len(m.Path), m.Path))
	b.WriteString(fmt.Sprintf(":%d", len(m.Args)))
	for _, arg := range m.Args {
		b.WriteString(fmt.Sprintf(":%d:%s", len(arg), arg))
	}
	b.WriteString(fmt.Sprintf(":%d", len(m.Env)))
	for _, env := range m.Env {
		b.WriteString(fmt.Sprintf(":%d:%s", len(env), env))
	}
	return b.String()
}

// LogSpec is details about the log that records a trace of execution.
type LogSpec struct {
	ProcessID   ProcessID
	StartTime   time.Time
	Compression timemachine.Compression
	BatchSize   int
}
