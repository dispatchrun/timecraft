package timemachine

import (
	"bytes"
	"math"
	"reflect"
	"testing"

	"github.com/stealthrocket/wazergo/wasm"
	"github.com/tetratelabs/wazero/api"
)

func TestMemoryInterceptor(t *testing.T) {
	for _, test := range []struct {
		name   string
		before []byte
		fn     func(*testing.T, api.Memory)
		after  []byte
	}{
		{
			name: "write uints 0xFF",
			fn: func(t *testing.T, memory api.Memory) {
				memory.WriteByte(0, ^byte(0))
				memory.WriteUint16Le(1, ^uint16(0))
				memory.WriteUint32Le(3, ^uint32(0))
				memory.WriteUint64Le(7, ^uint64(0))
			},
			before: bytes.Repeat([]byte{0}, 15),
			after:  bytes.Repeat([]byte{0xFF}, 15),
		},
		{
			name: "write uints asc",
			fn: func(t *testing.T, memory api.Memory) {
				memory.WriteByte(0, 1)
				memory.WriteUint16Le(1, 0x0302)
				memory.WriteUint32Le(3, 0x07060504)
				memory.WriteUint64Le(7, 0x0F0E0D0C0B0A0908)
			},
			before: bytes.Repeat([]byte{0xFF}, 15),
			after:  []byte{0x1, 0x2, 0x3, 0x4, 0x5, 0x6, 0x7, 0x8, 0x9, 0xA, 0xB, 0xC, 0xD, 0xE, 0xF},
		},
		{
			name: "write float",
			fn: func(t *testing.T, memory api.Memory) {
				memory.WriteFloat32Le(0, float32(-1))
				memory.WriteFloat64Le(4, math.Inf(-1))
			},
			before: bytes.Repeat([]byte{0}, 12),
			after:  []byte{0, 0, 0x80, 0xbf, 0, 0, 0, 0, 0, 0, 0xF0, 0xFF},
		},
		{
			name: "write string and bytes",
			fn: func(t *testing.T, memory api.Memory) {
				memory.Write(0, []byte("fox"))
				memory.Write(2, []byte("obar"))
				memory.WriteString(5, "rbaz")
			},
			before: bytes.Repeat([]byte{0}, 9),
			after:  []byte("foobarbaz"),
		},
		{
			name: "read + write",
			fn: func(t *testing.T, memory api.Memory) {
				b2, _ := memory.Read(0, 1)
				b1, _ := memory.Read(0, 3)
				copy(b1, "fox")
				memory.WriteByte(2, 'o')
				copy(b2, "b")
			},
			before: bytes.Repeat([]byte{0}, 3),
			after:  []byte("boo"),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			length := uint32(len(test.before))
			memory := wasm.NewFixedSizeMemory(length)
			b, _ := memory.Read(0, length)
			copy(b, test.before)

			mi := &MemoryInterceptor{Memory: memory}

			test.fn(t, mi)

			actual, _ := memory.Read(0, length)
			if !reflect.DeepEqual(actual, test.after) {
				t.Error("unexpected memory after running function")
				t.Logf("expect: %v", test.after)
				t.Logf("actual: %v", actual)
			}

			memoryAccess := mi.MemoryAccess()

			// Clear the memory to make sure that the changes below
			// refer to copies.
			for i := range actual {
				actual[i] = 0
			}

			// Check changes can be applied to input memory to reach the
			// same output state.
			actual = make([]byte, int(length))
			copy(actual, test.before)
			for _, m := range memoryAccess {
				copy(actual[m.Offset:], m.Memory)
			}
			if !reflect.DeepEqual(actual, test.after) {
				t.Error("unexpected memory after applying access to initial memory")
				t.Logf("expect: %v", test.after)
				t.Logf("actual: %v", actual)
			}
		})
	}
}
