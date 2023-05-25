package stream_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/stream"
)

func TestValues(t *testing.T) {
	values := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	reader := stream.NewReader(values...)

	read, err := stream.Values(stream.Iter(reader))
	assert.OK(t, err)
	assert.EqualAll(t, read, values)
}
