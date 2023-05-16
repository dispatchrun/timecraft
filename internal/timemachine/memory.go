package timemachine

import (
	"bytes"

	"github.com/tetratelabs/wazero/api"
)

// MemoryInterceptor intercepts writes to an api.Memory instance.
//
// After intercepting writes, the interceptor provides a set of
// ordered Mutations that can be applied to the input memory to
// get it in the same state as the output memory.
type MemoryInterceptor struct {
	api.Memory
	mutations []MemoryAccess
	reads     []int
}

// Mutations returns an ordered set of memory mutations.
//
// The slice is invalidated by any further method calls and so should
// not be retained.
func (m *MemoryInterceptor) Mutations() []MemoryAccess {
	m.compact()
	return m.mutations
}

// Reset resets the interceptor.
func (m *MemoryInterceptor) Reset(memory api.Memory) {
	m.Memory = memory
	m.mutations = m.mutations[:0]
	m.reads = m.reads[:0]
}

// Read reads bytes from memory.
//
// Any mutations to the slice are applied to the underlying memory.
func (m *MemoryInterceptor) Read(offset, length uint32) (b []byte, ok bool) {
	if b, ok = m.Memory.Read(offset, length); ok {
		m.capture(offset, length, 'r')
	}
	return
}

// WriteByte writes a byte to memory.
func (m *MemoryInterceptor) WriteByte(offset uint32, v byte) (ok bool) {
	if ok = m.WriteByte(offset, v); ok {
		m.capture(offset, 1, 'w')
	}
	return
}

// WriteUint16Le writes a uint16 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteUint16Le(offset uint32, v uint16) (ok bool) {
	if ok = m.WriteUint16Le(offset, v); ok {
		m.capture(offset, 2, 'w')
	}
	return
}

// WriteUint32Le writes a uint32 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteUint32Le(offset uint32, v uint32) (ok bool) {
	if ok = m.WriteUint32Le(offset, v); ok {
		m.capture(offset, 4, 'w')
	}
	return
}

// WriteFloat32Le writes a float32 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteFloat32Le(offset uint32, v float32) (ok bool) {
	if ok = m.WriteFloat32Le(offset, v); ok {
		m.capture(offset, 4, 'w')
	}
	return
}

// WriteUint64Le writes a uint64 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteUint64Le(offset uint32, v uint64) (ok bool) {
	if ok = m.WriteUint64Le(offset, v); ok {
		m.capture(offset, 8, 'w')
	}
	return
}

// WriteFloat64Le writes a float64 to memory in little endian byte order.
func (m *MemoryInterceptor) WriteFloat64Le(offset uint32, v float64) (ok bool) {
	if ok = m.WriteFloat64Le(offset, v); ok {
		m.capture(offset, 8, 'w')
	}
	return
}

// Write writes bytes to memory.
func (m *MemoryInterceptor) Write(offset uint32, v []byte) (ok bool) {
	if ok = m.Write(offset, v); ok {
		m.capture(offset, uint32(len(v)), 'w')
	}
	return
}

// WriteString writes a string to memory.
func (m *MemoryInterceptor) WriteString(offset uint32, v string) (ok bool) {
	if ok = m.WriteString(offset, v); ok {
		m.capture(offset, uint32(len(v)), 'w')
	}
	return
}

func (m *MemoryInterceptor) capture(offset, length uint32, mode int) {
	b, _ := m.Memory.Read(offset, length)
	if mode == 'r' {
		// Mutations may be applied to the slice returned by Read. We have to
		// assume at this stage that the slice may be mutated. Collect a copy
		// and mark the index so that we can check whether mutations occurred
		// later.
		m.reads = append(m.reads, len(m.mutations))
	}
	m.mutations = append(m.mutations, MemoryAccess{
		Memory: append(make([]byte, 0, len(b)), b...),
		Offset: offset,
	})
}

func (m *MemoryInterceptor) compact() {
	// Scan the Read slices. Check the same slice in memory to see if
	// mutations occurred. If they did, we must create a new mutation
	// at the *end* of the slice in case there are aliased writes later.
	for _, i := range m.reads {
		mut := &m.mutations[i]

		b, _ := m.Memory.Read(mut.Offset, uint32(len(mut.Memory)))

		if !bytes.Equal(b, mut.Memory) {
			m.mutations = append(m.mutations, MemoryAccess{
				Memory: b,
				Offset: mut.Offset,
			})
		}

		mut.Memory = nil // delete below
	}

	j := 0
	for i := 0; i < len(m.mutations); i++ {
		if m.mutations[i].Memory != nil {
			m.mutations[j] = m.mutations[i]
			j++
		}
	}
	m.mutations = m.mutations[:j]
}
