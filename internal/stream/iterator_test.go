package stream_test

import (
	"io"
	"testing"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
	"github.com/stealthrocket/timecraft/internal/stream"
)

func TestValues(t *testing.T) {
	values := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	reader := stream.NewReader(values...)

	read, err := stream.Values(stream.Iter(reader))
	assert.OK(t, err)
	assert.EqualAll(t, read, values)
}

func TestEOF(t *testing.T) {
	values := []int{0, 1, 2, 3}
	reader := chunks([][]int{{}, {0}, {}, {1, 2, 3}, {}})

	read, err := stream.Values(stream.Iter(reader))
	assert.OK(t, err)
	assert.EqualAll(t, read, values)
}

func chunks[T any](chunks [][]T) stream.Reader[T] {
	return &chunkedReader[T]{chunks: chunks}
}

type chunkedReader[T any] struct {
	chunks [][]T
}

func (r *chunkedReader[T]) Read(values []T) (int, error) {
	if len(r.chunks) == 0 {
		return 0, io.EOF
	}
	n := copy(values, r.chunks[0])
	if len(r.chunks[0]) == n {
		r.chunks = r.chunks[1:]
	} else {
		r.chunks[0] = r.chunks[0][n:]
	}
	return n, nil
}
