package timemachine

import (
	"bytes"
	"sync"

	"github.com/tetratelabs/wazero/api"
)

// MemoryInterceptor intercepts reads and writes to an api.Memory instance.
//
// After intercepting reads writes, the interceptor provides a set of
// captured []MemoryAccess that can be applied to the input memory to
// get it in the same state as the output memory.
type MemoryInterceptor struct {
	api.Memory
	access     []MemoryAccess
	readWrites []int
	buffer     []byte
}

// MemoryAccess returns an ordered set of memory reads and writes.
//
// The slice is invalidated by any further method calls and so should
// not be retained.
func (m *MemoryInterceptor) MemoryAccess() []MemoryAccess {
	m.observeWrites()
	return m.access
}

// Reset resets the interceptor.
func (m *MemoryInterceptor) Reset(memory api.Memory) {
	m.Memory = memory
	m.access = m.access[:0]
	m.readWrites = m.readWrites[:0]
	m.buffer = m.buffer[:0]
}

// ReadByte reads a byte from memory.
func (m *MemoryInterceptor) ReadByte(offset uint32) (v byte, ok bool) {
	if v, ok = m.Memory.ReadByte(offset); ok {
		m.capture(offset, 1, 'r')
	}
	return
}

// ReadUint16Le reads a uint16 from memory in little endian byte order.
func (m *MemoryInterceptor) ReadUint16Le(offset uint32) (v uint16, ok bool) {
	if v, ok = m.Memory.ReadUint16Le(offset); ok {
		m.capture(offset, 2, 'r')
	}
	return
}

// ReadUint32Le reads a uint32 from memory in little endian byte order.
func (m *MemoryInterceptor) ReadUint32Le(offset uint32) (v uint32, ok bool) {
	if v, ok = m.Memory.ReadUint32Le(offset); ok {
		m.capture(offset, 4, 'r')
	}
	return
}

// ReadFloat32Le reads a float32 from memory in little endian byte order.
func (m *MemoryInterceptor) ReadFloat32Le(offset uint32) (v float32, ok bool) {
	if v, ok = m.Memory.ReadFloat32Le(offset); ok {
		m.capture(offset, 4, 'r')
	}
	return
}

// ReadUint64Le reads a uint64 from memory in little endian byte order.
func (m *MemoryInterceptor) ReadUint64Le(offset uint32) (v uint64, ok bool) {
	if v, ok = m.Memory.ReadUint64Le(offset); ok {
		m.capture(offset, 8, 'r')
	}
	return
}

// ReadFloat64Le reads a float64 from memory in little endian byte order.
func (m *MemoryInterceptor) ReadFloat64Le(offset uint32) (v float64, ok bool) {
	if v, ok = m.Memory.ReadFloat64Le(offset); ok {
		m.capture(offset, 8, 'r')
	}
	return
}

// Read reads bytes from memory.
//
// Any access to the slice are applied to the underlying memory.
func (m *MemoryInterceptor) Read(offset, length uint32) (b []byte, ok bool) {
	if b, ok = m.Memory.Read(offset, length); ok {
		m.capture(offset, length, 'r'+'w')
	}
	return
}

// WriteByte writes a byte to memory.
func (m *MemoryInterceptor) WriteByte(offset uint32, v byte) (ok bool) {
	if ok = m.Memory.WriteByte(offset, v); ok {
		m.capture(offset, 1, 'w')
	}
	return
}

// WriteUint16Le writes a uint16 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteUint16Le(offset uint32, v uint16) (ok bool) {
	if ok = m.Memory.WriteUint16Le(offset, v); ok {
		m.capture(offset, 2, 'w')
	}
	return
}

// WriteUint32Le writes a uint32 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteUint32Le(offset uint32, v uint32) (ok bool) {
	if ok = m.Memory.WriteUint32Le(offset, v); ok {
		m.capture(offset, 4, 'w')
	}
	return
}

// WriteFloat32Le writes a float32 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteFloat32Le(offset uint32, v float32) (ok bool) {
	if ok = m.Memory.WriteFloat32Le(offset, v); ok {
		m.capture(offset, 4, 'w')
	}
	return
}

// WriteUint64Le writes a uint64 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteUint64Le(offset uint32, v uint64) (ok bool) {
	if ok = m.Memory.WriteUint64Le(offset, v); ok {
		m.capture(offset, 8, 'w')
	}
	return
}

// WriteFloat64Le writes a float64 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteFloat64Le(offset uint32, v float64) (ok bool) {
	if ok = m.Memory.WriteFloat64Le(offset, v); ok {
		m.capture(offset, 8, 'w')
	}
	return
}

// Write writes bytes to memory.
func (m *MemoryInterceptor) Write(offset uint32, v []byte) (ok bool) {
	if ok = m.Memory.Write(offset, v); ok {
		m.capture(offset, uint32(len(v)), 'w')
	}
	return
}

// WriteString writes a string to memory.
func (m *MemoryInterceptor) WriteString(offset uint32, v string) (ok bool) {
	if ok = m.Memory.WriteString(offset, v); ok {
		m.capture(offset, uint32(len(v)), 'w')
	}
	return
}

func (m *MemoryInterceptor) capture(offset, length uint32, mode int) {
	b, _ := m.Memory.Read(offset, length)
	if mode == 'r'+'w' {
		// Mutations may be applied to the slice returned by Read. We have to
		// assume at this stage that the slice may be mutated. Collect a copy
		// and mark the index so that we can check whether access occurred
		// later.
		m.readWrites = append(m.readWrites, len(m.access))
	}
	m.buffer = append(m.buffer, b...)
	m.access = append(m.access, MemoryAccess{
		Memory: m.buffer[len(m.buffer)-len(b):],
		Offset: offset,
	})
}

func (m *MemoryInterceptor) observeWrites() {
	// Scan the Read slices. Check the same slice in memory to see if
	// access occurred. If they did, we must create a new mutation
	// at the *end* of the slice in case there are other aliased writes
	// after the read.
	for _, i := range m.readWrites {
		a := &m.access[i]
		offset, length := a.Offset, uint32(len(a.Memory))
		if b, _ := m.Memory.Read(offset, length); !bytes.Equal(b, a.Memory) {
			m.capture(offset, length, 'w')
		}
	}
	m.readWrites = m.readWrites[:0]
}

// MemoryInterceptorModule is an api.Module wrapper for a MemoryInterceptor
// implementation of api.Memory.
type MemoryInterceptorModule struct {
	api.Module
	mem *MemoryInterceptor
}

func (m *MemoryInterceptorModule) Memory() api.Memory {
	return m.mem
}

var memoryInterceptorModulePool sync.Pool // *MemoryInterceptorModule

// GetMemoryInterceptorModule gets a memory interceptor module from the pool.
func GetMemoryInterceptorModule(module api.Module) *MemoryInterceptorModule {
	if p := memoryInterceptorModulePool.Get(); p != nil {
		m := p.(*MemoryInterceptorModule)
		m.Module = module
		m.mem.Reset(module.Memory())
		return m
	}
	return &MemoryInterceptorModule{
		Module: module,
		mem: &MemoryInterceptor{
			Memory: module.Memory(),
		},
	}
}

// PutMemoryInterceptorModule returns a memory interceptor module to the pool.
func PutMemoryInterceptorModule(m *MemoryInterceptorModule) {
	m.mem.Reset(nil)
	memoryInterceptorModulePool.Put(m)
}
