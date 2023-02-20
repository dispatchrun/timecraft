package timecraft

import (
	"bytes"
	"context"
	"encoding/binary"
	"math"
	"time"
	"unsafe"

	"github.com/stealthrocket/plugins/wasm"
	"github.com/tetratelabs/wazero/api"
	"golang.org/x/exp/slices"
)

type Record struct {
	Walltime int64          `parquet:"walltime,delta"`
	Duration int64          `parquet:"duration,delta"`
	Module   string         `parquet:"module,dict"`
	Function string         `parquet:"function,dict"`
	Params   []uint64       `parquet:"params,plain"`
	Results  []uint64       `parquet:"results,plain"`
	Memory   []MemoryRecord `parquet:"memory"`
}

type MemoryRecord struct {
	Offset uint32 `parquet:"offset,plain"`
	Length uint32 `parquet:"length,plain"`
	Bytes  []byte `parquet:"bytes,zstd"`
	Write  bool   `parquet:"write"`
}

func Capture[T wasm.Module](epoch time.Time, capture func(context.Context, *Record)) wasm.Decorator[T] {
	return wasm.DecoratorFunc[T](func(moduleName string, fn wasm.Function[T]) wasm.Function[T] {
		if capture == nil {
			return fn
		}
		function := fn.Func
		numParams := 0
		numResults := 0

		for _, v := range fn.Params {
			numParams += len(v.ValueTypes())
		}
		for _, v := range fn.Results {
			numResults += len(v.ValueTypes())
		}

		fn.Func = func(this T, ctx context.Context, mod api.Module, stack []uint64) {
			now := time.Now()
			rec := &Record{
				Module:   moduleName,
				Function: fn.Name,
				Params:   slices.Clone(stack[:numParams]),
			}
			defer func() {
				rec.Walltime = int64(now.Sub(epoch))
				rec.Duration = int64(time.Since(now))
				rec.Results = slices.Clone(stack[:numResults])
				capture(ctx, rec)
			}()
			module := &moduleWithMemoryRecords{
				Module: mod,
				mem: memory{
					rec: rec,
					mem: mod.Memory(),
				},
			}
			defer module.mem.observeLastWrite()
			function(this, ctx, module, stack)
		}
		return fn
	})
}

type moduleWithMemoryRecords struct {
	api.Module
	mem memory
}

func (m *moduleWithMemoryRecords) Memory() api.Memory { return &m.mem }

type memory struct {
	rec *Record
	mem api.Memory
}

func (m *memory) Definition() api.MemoryDefinition { return m.mem.Definition() }

func (m *memory) Size() uint32 { return m.mem.Size() }

func (m *memory) Grow(size uint32) (uint32, bool) { return m.mem.Grow(size) }

func (m *memory) ReadByte(offset uint32) (byte, bool) {
	b, ok := m.mem.ReadByte(offset)
	m.read(offset, 1, []byte{b}, ok)
	return b, ok
}

func (m *memory) ReadUint16Le(offset uint32) (uint16, bool) {
	v, ok := m.mem.ReadUint16Le(offset)
	b := [2]byte{}
	binary.LittleEndian.PutUint16(b[:], v)
	m.read(offset, 2, b[:], ok)
	return v, ok
}

func (m *memory) ReadUint32Le(offset uint32) (uint32, bool) {
	v, ok := m.mem.ReadUint32Le(offset)
	b := [4]byte{}
	binary.LittleEndian.PutUint32(b[:], v)
	m.read(offset, 4, b[:], ok)
	return v, ok
}

func (m *memory) ReadUint64Le(offset uint32) (uint64, bool) {
	v, ok := m.mem.ReadUint64Le(offset)
	b := [8]byte{}
	binary.LittleEndian.PutUint64(b[:], v)
	m.read(offset, 8, b[:], ok)
	return v, ok
}

func (m *memory) ReadFloat32Le(offset uint32) (float32, bool) {
	v, ok := m.mem.ReadFloat32Le(offset)
	b := [4]byte{}
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	m.read(offset, 4, b[:], ok)
	return v, ok
}

func (m *memory) ReadFloat64Le(offset uint32) (float64, bool) {
	v, ok := m.mem.ReadFloat64Le(offset)
	b := [8]byte{}
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	m.read(offset, 8, b[:], ok)
	return v, ok
}

func (m *memory) Read(offset, length uint32) ([]byte, bool) {
	b, ok := m.mem.Read(offset, length)
	m.read(offset, length, b, ok)
	return b, ok
}

func (m *memory) WriteByte(offset uint32, value byte) bool {
	ok := m.mem.WriteByte(offset, value)
	m.write(offset, 1, []byte{value}, ok)
	return ok
}

func (m *memory) WriteUint16Le(offset uint32, value uint16) bool {
	ok := m.mem.WriteUint16Le(offset, value)
	b := [2]byte{}
	binary.LittleEndian.PutUint16(b[:], value)
	m.write(offset, 2, b[:], ok)
	return ok
}

func (m *memory) WriteUint32Le(offset uint32, value uint32) bool {
	ok := m.mem.WriteUint32Le(offset, value)
	b := [4]byte{}
	binary.LittleEndian.PutUint32(b[:], value)
	m.write(offset, 4, b[:], ok)
	return ok
}

func (m *memory) WriteUint64Le(offset uint32, value uint64) bool {
	ok := m.mem.WriteUint64Le(offset, value)
	b := [8]byte{}
	binary.LittleEndian.PutUint64(b[:], value)
	m.write(offset, 8, b[:], ok)
	return ok
}

func (m *memory) WriteFloat32Le(offset uint32, value float32) bool {
	ok := m.mem.WriteFloat32Le(offset, value)
	b := [4]byte{}
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(value))
	m.write(offset, 4, b[:], ok)
	return ok
}

func (m *memory) WriteFloat64Le(offset uint32, value float64) bool {
	ok := m.mem.WriteFloat64Le(offset, value)
	b := [8]byte{}
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(value))
	m.write(offset, 8, b[:], ok)
	return ok
}

func (m *memory) Write(offset uint32, value []byte) bool {
	ok := m.mem.Write(offset, value)
	m.write(offset, uint32(len(value)), value, ok)
	return ok
}

func (m *memory) WriteString(offset uint32, value string) bool {
	ok := m.mem.WriteString(offset, value)
	m.write(offset, uint32(len(value)), unsafe.Slice(unsafe.StringData(value), len(value)), ok)
	return true
}

func (m *memory) read(offset, length uint32, data []byte, ok bool) {
	m.observe(offset, length, data, ok, false)
}

func (m *memory) write(offset, length uint32, data []byte, ok bool) {
	m.observe(offset, length, data, ok, true)
}

func (m *memory) observe(offset, length uint32, data []byte, ok, write bool) {
	m.observeLastWrite()

	if ok {
		data = slices.Clone(data)
	} else {
		data = nil
	}

	m.rec.Memory = append(m.rec.Memory, MemoryRecord{
		Offset: offset,
		Length: length,
		Bytes:  data,
		Write:  write,
	})
}

func (m *memory) observeLastWrite() {
	// Writes may be performed by calling api.Memory.Read and mutating the
	// returned byte slice. For this reason, we validate that the last memory
	// access did not mutate the memory area before recording the next access
	// to ensure we maintain the order of memory reads and writes. If the last
	// memory region was mutated, we record a memory write firsts before adding
	// the next memory access.
	if len(m.rec.Memory) > 0 {
		i := len(m.rec.Memory) - 1
		r := &m.rec.Memory[i]

		b, ok := m.mem.Read(r.Offset, r.Length)
		if ok {
			if !bytes.Equal(b, r.Bytes) {
				m.rec.Memory = append(m.rec.Memory, MemoryRecord{
					Offset: r.Offset,
					Length: r.Length,
					Bytes:  slices.Clone(b),
					Write:  true,
				})
			}
		}
	}
}
