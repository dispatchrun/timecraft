package sandbox

import (
	"bytes"
	"sync"
	"testing"
	"testing/iotest"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
)

func TestPipeInput(t *testing.T) {
	b := bytes.Repeat([]byte("1234567890"), 1000)
	p := newPipe(new(sync.Mutex))

	go func() {
		w := inputWriteCloser{p}
		n, err := w.Write(b)
		assert.OK(t, w.Close())
		assert.OK(t, err)
		assert.Equal(t, n, len(b))
	}()

	assert.OK(t, iotest.TestReader(inputReadCloser{p}, b))
}

func TestPipeOutput(t *testing.T) {
	b := bytes.Repeat([]byte("1234567890"), 1000)
	p := newPipe(new(sync.Mutex))

	go func() {
		w := outputWriteCloser{p}
		n, err := w.Write(b)
		assert.OK(t, w.Close())
		assert.OK(t, err)
		assert.Equal(t, n, len(b))
	}()

	assert.OK(t, iotest.TestReader(outputReadCloser{p}, b))
}
