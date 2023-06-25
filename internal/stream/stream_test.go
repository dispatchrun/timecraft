package stream_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
	"github.com/stealthrocket/timecraft/internal/stream"
)

func TestReadAll(t *testing.T) {
	values := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	reader := stream.NewReader(values...)

	read, err := stream.ReadAll(reader)
	assert.OK(t, err)
	assert.EqualAll(t, read, values)
}
