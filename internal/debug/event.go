package debug

import (
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// Event is an event generated during execution of a WebAssembly module,
// such as a function call.
type Event interface {
	event()
}

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

// WASICallBefore is an event that's triggered before a WASI
// host function is called.
type WASICallBefore struct {
	wasicall.Syscall
}

func (*WASICallBefore) event() {}

// WASICallAfter is an event that's triggered after a WASI
// host function is called.
type WASICallAfter struct {
	wasicall.Syscall
}

func (*WASICallAfter) event() {}
