package timemachine

import (
	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
)

// FunctionCall is read-only details about a host function call.
type FunctionCall struct {
	function *Function
	call     logsegment.FunctionCall
}

// MemoryAccess is memory captured from the WebAssembly module.
type MemoryAccess struct {
	Memory []byte
	Offset uint32
}

// NumParams returns the number of function parameters.
func (c *FunctionCall) NumParams() int {
	return c.function.ParamCount
}

// Param returns the function parameter at the specified index.
func (c *FunctionCall) Param(i int) uint64 {
	return c.call.Stack(i)
}

// NumResults returns the number of function return values.
func (c *FunctionCall) NumResults() int {
	return c.function.ResultCount
}

// Result returns the function return value at the specified index.
func (c *FunctionCall) Result(i int) uint64 {
	return c.call.Stack(i + c.NumParams())
}

// NumMemoryAccess returns the number of memory accesses.
func (c *FunctionCall) NumMemoryAccess() int {
	return c.call.MemoryAccessLength()
}

// MemoryAccess returns the memory access at the specified index.
func (c *FunctionCall) MemoryAccess(i int) MemoryAccess {
	memory := c.call.MemoryBytes()
	var ma logsegment.MemoryAccess
	if !c.call.MemoryAccess(&ma, i) {
		panic("invalid memory access")
	}
	offset, length := ma.IndexOffset(), ma.Length()
	if offset+length < offset || offset+length > uint32(len(memory)) {
		panic("invalid memory access")
	}
	return MemoryAccess{
		Memory: memory[offset : offset+length : offset+length],
		Offset: ma.Offset(),
	}
}

// FunctionCallBuilder is a builder for function calls.
type FunctionCallBuilder struct {
	function *Function
	builder  *flatbuffers.Builder
	finished bool
}

// Reset resets the builder.
func (f *FunctionCallBuilder) Reset(function *Function) {
	f.builder.Reset()
	f.finished = false
}

// Bytes returns the serialized representation of the function call.
func (f *FunctionCallBuilder) Bytes() []byte {
	return f.builder.FinishedBytes()
}
