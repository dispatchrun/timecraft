package timemachine

import (
	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
	"github.com/tetratelabs/wazero/api"
)

// FunctionCall is read-only details about a host function call.
type FunctionCall struct {
	function *Function
	call     logsegment.FunctionCall
}

// MakeFunctionCall makes a function call.
func MakeFunctionCall(function *Function, b []byte) FunctionCall {
	call := logsegment.GetRootAsFunctionCall(b, 0)
	return FunctionCall{
		function: function,
		call:     *call,
	}
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
	memory   MemoryInterceptor
	stack    []uint64
	finished bool
}

// Reset resets the builder.
func (b *FunctionCallBuilder) Reset(function *Function) {
	b.function = function
	if cap(b.stack) < function.ParamCount+function.ResultCount {
		b.stack = make([]uint64, function.ParamCount+function.ResultCount)
	} else {
		b.stack = b.stack[:function.ParamCount+function.ResultCount]
	}
	if b.builder == nil {
		b.builder = flatbuffers.NewBuilder(defaultBufferSize)
	} else {
		b.builder.Reset()
	}
	b.memory.Reset(nil)
	b.finished = false
}

func (b *FunctionCallBuilder) SetParams(params []uint64) {
	if b.finished || b.function == nil {
		panic("builder must be reset before params can be set")
	}
	if len(params) != b.function.ParamCount {
		panic("param count mismatch")
	}
	copy(b.stack, params)
}

func (b *FunctionCallBuilder) SetResults(results []uint64) {
	if b.finished || b.function == nil {
		panic("builder must be reset before results can be set")
	}
	if len(results) != b.function.ResultCount {
		panic("result count mismatch")
	}
	copy(b.stack[b.function.ParamCount:], results)
}

// MemoryInterceptor returns a helper to intercept memory.
func (b *FunctionCallBuilder) MemoryInterceptor(mem api.Memory) *MemoryInterceptor {
	if b.finished || b.function == nil {
		panic("builder must be reset before memory interceptor can be used")
	}
	b.memory.Reset(mem)
	return &b.memory
}

// Bytes returns the serialized representation of the function call.
func (b *FunctionCallBuilder) Bytes() []byte {
	if !b.finished {
		b.build()
		b.finished = true
	}
	return b.builder.FinishedBytes()
}

func (b *FunctionCallBuilder) build() {
	if b.function == nil || b.builder == nil {
		panic("builder is not initialized")
	}
	b.builder.StartVector(flatbuffers.SizeUint64, len(b.stack), flatbuffers.SizeUint64)
	for i := len(b.stack) - 1; i >= 0; i-- {
		b.builder.PrependUint64(b.stack[i])
	}
	stackOffset := b.builder.EndVector(len(b.stack))

	b.memory.observeWrites()
	memoryAccess := b.memory.access
	memoryBuffer := b.memory.buffer
	memoryOffset := b.builder.CreateByteVector(memoryBuffer)

	logsegment.FunctionCallStartMemoryAccessVector(b.builder, len(memoryAccess))

	b.builder.Prep(12*len(memoryAccess), 0)
	for i := len(memoryAccess) - 1; i >= 0; i-- {
		m := &memoryAccess[i]
		b.builder.PlaceUint32(m.index)
		b.builder.PlaceUint32(m.offset)
		b.builder.PlaceUint32(m.length)
	}
	memoryAccessOffset := b.builder.EndVector(len(memoryAccess))

	logsegment.FunctionCallStart(b.builder)
	logsegment.FunctionCallAddStack(b.builder, stackOffset)
	logsegment.FunctionCallAddMemoryAccess(b.builder, memoryAccessOffset)
	logsegment.FunctionCallAddMemory(b.builder, memoryOffset)
	logsegment.FinishFunctionCallBuffer(b.builder, logsegment.FunctionCallEnd(b.builder))
}
