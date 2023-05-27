package debug

import (
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// Event is an event generated during execution of a WebAssembly module.
type Event interface {
	event()
}

// ModuleBefore is an event that's triggered before a module is executed.
type ModuleBefore struct{}

func (*ModuleBefore) event() {}

// ModuleAfter is an event that's triggered after a module has been executed.
type ModuleAfter struct{}

func (*ModuleAfter) event() {}

// FunctionCallBefore is an event that's triggered before a function is
// called in a WebAssembly module.
type FunctionCallBefore struct {
	Module        api.Module
	Function      api.FunctionDefinition
	Params        []uint64
	StackIterator experimental.StackIterator
}

func (*FunctionCallBefore) event() {}

// FunctionCallAfter is an event that's triggered after a function is
// called in a WebAssembly module.
type FunctionCallAfter struct {
	Module   api.Module
	Function api.FunctionDefinition
	Results  []uint64
}

func (*FunctionCallAfter) event() {}

// FunctionCallAbort is an event that's triggered when a function call is
// aborted in a WebAssembly module.
type FunctionCallAbort struct {
	Module   api.Module
	Function api.FunctionDefinition
	Error    error
}

func (*FunctionCallAbort) event() {}

// WASICallBefore is an event that's triggered before a WASI host function is
// called in a WebAssembly module.
type WASICallBefore struct {
	wasicall.Syscall
}

func (*WASICallBefore) event() {}

// WASICallAfter is an event that's triggered before a WASI host function is
// called in a WebAssembly module.
type WASICallAfter struct {
	wasicall.Syscall
}

func (*WASICallAfter) event() {}
