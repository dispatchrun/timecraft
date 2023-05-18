package timemachine

import (
	"io"
	"unsafe"

	flatbuffers "github.com/google/flatbuffers/go"
)

func prependUint32Vector(b *flatbuffers.Builder, values []uint32) flatbuffers.UOffsetT {
	b.StartVector(4, len(values), 4)
	for i := len(values) - 1; i >= 0; i-- {
		b.PrependUint32(values[i])
	}
	return b.EndVector(len(values))
}

func prependUint64Vector(b *flatbuffers.Builder, values []uint64) flatbuffers.UOffsetT {
	b.StartVector(8, len(values), 8)
	for i := len(values) - 1; i >= 0; i-- {
		b.PrependUint64(values[i])
	}
	return b.EndVector(len(values))
}

func uint64Vector(table flatbuffers.Table, field flatbuffers.VOffsetT) []uint64 {
	offset := flatbuffers.UOffsetT(table.Offset(field))
	if offset == 0 {
		return nil
	}
	length := table.VectorLen(offset)
	vector := table.Vector(offset)
	return unsafe.Slice((*uint64)(unsafe.Pointer(&table.Bytes[vector])), length)
}

func readAll(input io.ReaderAt, size int64) ([]byte, error) {
	b := make([]byte, size)

	n, err := input.ReadAt(b, 0)
	if n < len(b) {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
	}

	return b[:n], err
}
