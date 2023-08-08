package sandbox_test

import (
	"io/fs"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"golang.org/x/sys/unix"
)

func TestReadDirent(t *testing.T) {
	b := make([]byte, 256)
	n := 0
	n += sandbox.WriteDirent(b[n:], fs.ModeDir, 1, uint64(n), ".")
	n += sandbox.WriteDirent(b[n:], fs.ModeDir, 2, uint64(n), "..")
	n += sandbox.WriteDirent(b[n:], 0, 3, uint64(n), "hello")
	n += sandbox.WriteDirent(b[n:], fs.ModeSymlink, 4, uint64(n), "world")
	b = b[:n]

	n, typ, ino, _, name, err := sandbox.ReadDirent(b)
	assert.OK(t, err)
	assert.Equal(t, typ, fs.ModeDir)
	assert.Equal(t, ino, 1)
	assert.Equal(t, string(name), ".")
	b = b[n:]

	n, typ, ino, _, name, err = sandbox.ReadDirent(b)
	assert.OK(t, err)
	assert.Equal(t, typ, fs.ModeDir)
	assert.Equal(t, ino, 2)
	assert.Equal(t, string(name), "..")
	b = b[n:]

	n, typ, ino, _, name, err = sandbox.ReadDirent(b)
	assert.OK(t, err)
	assert.Equal(t, typ, 0)
	assert.Equal(t, ino, 3)
	assert.Equal(t, string(name), "hello")
	b = b[n:]

	n, typ, ino, _, name, err = sandbox.ReadDirent(b)
	assert.OK(t, err)
	assert.Equal(t, typ, fs.ModeSymlink)
	assert.Equal(t, ino, 4)
	assert.Equal(t, string(name), "world")
	b = b[n:]
	assert.Equal(t, len(b), 0)
}

func TestWriteDirent(t *testing.T) {
	b := make([]byte, 256)
	n := 0
	n += sandbox.WriteDirent(b[n:], fs.ModeDir, 1, uint64(n), ".")
	n += sandbox.WriteDirent(b[n:], fs.ModeDir, 2, uint64(n), "..")
	n += sandbox.WriteDirent(b[n:], 0, 3, uint64(n), "hello")
	n += sandbox.WriteDirent(b[n:], fs.ModeSymlink, 4, uint64(n), "world")

	consumed, count, newnames := unix.ParseDirent(b, 4, nil)
	assert.Equal(t, consumed, n)
	assert.Equal(t, count, 2)
	assert.Equal(t, newnames[0], "hello")
	assert.Equal(t, newnames[1], "world")
}

func BenchmarkWriteDirent(b *testing.B) {
	buf := make([]byte, 256)

	for i := 0; i < b.N; i++ {
		sandbox.WriteDirent(buf, 0, 3, 0, "hello")
	}
}
