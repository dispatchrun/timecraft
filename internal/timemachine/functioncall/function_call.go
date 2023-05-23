package functioncall

import (
	"io"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
	"github.com/stealthrocket/timecraft/internal/buffer"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/tetratelabs/wazero/api"
)

// FunctionCall is read-only details about a host function call.
type FunctionCall struct {
	function timemachine.Function
	call     logsegment.FunctionCall
}

// MakeFunctionCall creates a function call from the specified buffer.
//
// The buffer must live as long as the function call.
func MakeFunctionCall(function timemachine.Function, buffer []byte) (f FunctionCall) {
	f.Reset(function, buffer)
	return
}

// Reset resets the function call.
func (f *FunctionCall) Reset(function timemachine.Function, buffer []byte) {
	f.function = function
	f.call = *logsegment.GetRootAsFunctionCall(buffer, 0)
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
	var access logsegment.MemoryAccess

	if !c.call.MemoryAccess(&access, i) {
		panic("invalid memory access")
	}
	head := access.IndexOffset()
	tail := uint32(c.call.MemoryLength())
	if i < c.call.MemoryAccessLength()-1 {
		var next logsegment.MemoryAccess
		if !c.call.MemoryAccess(&next, i+1) {
			panic("invalid memory access")
		}
		nextIndex := next.IndexOffset()
		if nextIndex < head || nextIndex > tail {
			panic("oob")
		}
		tail = nextIndex
	}
	return MemoryAccess{
		Memory: memory[head:tail:tail],
		Offset: access.Offset(),
	}
}

// FunctionCallBuilder is a builder for function calls.
type FunctionCallBuilder struct {
	function *timemachine.Function
	builder  *flatbuffers.Builder
	memory   MemoryInterceptor
	stack    []uint64
	finished bool
}

// Reset resets the builder.
func (b *FunctionCallBuilder) Reset(function *timemachine.Function) {
	b.function = function
	if cap(b.stack) < function.ParamCount+function.ResultCount {
		b.stack = make([]uint64, function.ParamCount+function.ResultCount)
	} else {
		b.stack = b.stack[:function.ParamCount+function.ResultCount]
	}
	if b.builder == nil {
		b.builder = flatbuffers.NewBuilder(buffer.DefaultSize)
	} else {
		b.builder.Reset()
	}
	b.memory.Reset(nil)
	b.finished = false
}

// SetParams sets the parameters from the stack.
func (b *FunctionCallBuilder) SetParams(params []uint64) {
	if b.finished || b.function == nil {
		panic("builder must be reset before params can be set")
	}
	if len(params) != b.function.ParamCount {
		panic("param count mismatch")
	}
	copy(b.stack, params)
}

// SetResults sets the return values for the stack.
func (b *FunctionCallBuilder) SetResults(results []uint64) {
	if b.finished || b.function == nil {
		panic("builder must be reset before results can be set")
	}
	if len(results) != b.function.ResultCount {
		panic("result count mismatch")
	}
	copy(b.stack[b.function.ParamCount:], results)
}

// AddMemoryAccess adds a memory access.
//
// It's not possible to add memory accesses like this when using the
// bundled memory interceptor.
func (b *FunctionCallBuilder) AddMemoryAccess(memoryAccess MemoryAccess) {
	if b.finished || b.function == nil {
		panic("builder must be reset before memory can be added")
	}
	if b.memory.Memory != nil {
		panic("builder has already been configured to intercept memory elsewhere")
	}
	b.memory.add(memoryAccess.Offset, memoryAccess.Memory)
}

// MemoryInterceptor returns a helper to intercept memory.
//
// It's not possible to use AddMemoryAccess when using the memory interceptor.
func (b *FunctionCallBuilder) MemoryInterceptor(mem api.Memory) *MemoryInterceptor {
	if b.finished || b.function == nil {
		panic("builder must be reset before memory interceptor can be used")
	}
	if len(b.memory.buffer) != 0 {
		panic("builder has already had memory added")
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

// Write writes the serialized representation of the function call
// to the specified writer.
func (b *FunctionCallBuilder) Write(w io.Writer) (int, error) {
	return w.Write(b.Bytes())
}

func (b *FunctionCallBuilder) build() {
	if b.function == nil {
		panic("builder is not initialized")
	}
	if b.builder == nil {
		b.builder = flatbuffers.NewBuilder(buffer.DefaultSize)
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

	b.builder.Prep(8*len(memoryAccess), 0)
	for i := len(memoryAccess) - 1; i >= 0; i-- {
		m := &memoryAccess[i]
		if i < len(memoryAccess)-1 && m.index > memoryAccess[i+1].index {
			panic("index invariant does not hold")
		}
		b.builder.PlaceUint32(m.index)
		b.builder.PlaceUint32(m.offset)
	}
	memoryAccessOffset := b.builder.EndVector(len(memoryAccess))

	logsegment.FunctionCallStart(b.builder)
	logsegment.FunctionCallAddStack(b.builder, stackOffset)
	logsegment.FunctionCallAddMemoryAccess(b.builder, memoryAccessOffset)
	logsegment.FunctionCallAddMemory(b.builder, memoryOffset)
	logsegment.FinishFunctionCallBuffer(b.builder, logsegment.FunctionCallEnd(b.builder))
}
