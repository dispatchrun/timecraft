package debug

import (
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// Event is an event generated during execution of a WebAssembly module.
type Event interface {
	event()
}

// ModuleBeforeEvent is an event that's triggered before a module is executed.
type ModuleBeforeEvent struct {
	Module wazero.CompiledModule
}

func (*ModuleBeforeEvent) event() {}

// ModuleAfterEvent is an event that's triggered after a module has been
// executed.
type ModuleAfterEvent struct {
	Error any
}

func (*ModuleAfterEvent) event() {}

// FunctionCallBeforeEvent is an event that's triggered before a function is
// called in a WebAssembly module.
type FunctionCallBeforeEvent struct {
	Module        api.Module
	Function      api.FunctionDefinition
	Params        []uint64
	StackIterator experimental.StackIterator
}

func (*FunctionCallBeforeEvent) event() {}

// FunctionCallAfterEvent is an event that's triggered after a function is
// called in a WebAssembly module.
type FunctionCallAfterEvent struct {
	Module   api.Module
	Function api.FunctionDefinition
	Results  []uint64
}

func (*FunctionCallAfterEvent) event() {}

// FunctionCallAbortEvent is an event that's triggered when a function call is
// aborted in a WebAssembly module.
type FunctionCallAbortEvent struct {
	Module   api.Module
	Function api.FunctionDefinition
	Error    error
}

func (*FunctionCallAbortEvent) event() {}

// SystemCallBeforeEvent is an event that's triggered before a WASI host
// function is called in a WebAssembly module.
type SystemCallBeforeEvent struct {
	wasicall.Syscall
}

func (*SystemCallBeforeEvent) event() {}

// SystemCallAfterEvent is an event that's triggered before a WASI host
// function is called in a WebAssembly module.
type SystemCallAfterEvent struct {
	wasicall.Syscall
}

func (*SystemCallAfterEvent) event() {}
