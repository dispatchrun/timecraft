package sandbox

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
)

func TestRingBuffer(t *testing.T) {
	b := makeRingBuffer[byte](20)
	n := b.write([]byte("hello world"))
	v := make([]byte, 32)
	assert.Equal(t, n, 11)

	for _, c := range []byte("hello world") {
		n := b.read(v[:1])
		assert.Equal(t, n, 1)
		assert.Equal(t, v[0], c)
	}

	n = b.write([]byte("12345678901234567890"))
	assert.Equal(t, n, 20)

	n = b.read(v)
	assert.Equal(t, n, 20)
	assert.Equal(t, string(v[:20]), "12345678901234567890")

	n = b.read(v)
	assert.Equal(t, n, 0)

	b.write([]byte{0})
	for i := 0; i < 100; i++ {
		assert.Equal(t, b.write([]byte{byte(i) + 1}), 1)
		assert.Equal(t, b.read(v[:1]), 1)
		assert.Equal(t, v[0], byte(i))
	}
}
