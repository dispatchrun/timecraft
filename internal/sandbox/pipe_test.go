package sandbox

import (
	"bytes"
	"context"
	"io"
	"sync"
	"testing"
	"testing/iotest"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/wasi-go"
)

func TestPipeRead(t *testing.T) {
	b := bytes.Repeat([]byte("1234567890"), 1000)
	p := newPipe(new(sync.Mutex))

	go func() {
		defer p.Close()
		w := &pipeWriter{p}
		n, err := w.Write(b)
		assert.OK(t, err)
		assert.Equal(t, n, len(b))
	}()

	assert.OK(t, iotest.TestReader(p, b))
}

func TestPipeWrite(t *testing.T) {
	b := bytes.Repeat([]byte("1234567890"), 1000)
	p := newPipe(new(sync.Mutex))

	go func() {
		defer p.Close()
		n, err := p.Write(b)
		assert.OK(t, err)
		assert.Equal(t, n, len(b))
	}()

	assert.OK(t, iotest.TestReader(&pipeReader{p}, b))
}

type FDReader interface {
	FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno)
}

type pipeReader struct {
	FDReader
}

func (r *pipeReader) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	size, errno := r.FDRead(context.Background(), []wasi.IOVec{b})
	if errno != wasi.ESUCCESS {
		return int(size), errno
	}
	if size == 0 {
		return 0, io.EOF
	}
	return int(size), nil
}

type FDWriter interface {
	FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno)
}

type pipeWriter struct {
	FDWriter
}

func (w *pipeWriter) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	n := 0
	for n < len(b) {
		size, errno := w.FDWrite(context.Background(), []wasi.IOVec{b[n:]})
		n += int(size)
		if errno != wasi.ESUCCESS {
			return n, errno
		}
	}
	return n, nil
}
