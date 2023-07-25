package sandbox

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

func TestSocketRefCount(t *testing.T) {
	var lastCloseFD int
	var closeFD = func(fd int) { lastCloseFD = fd }

	t.Run("close with zero ref count", func(t *testing.T) {
		var s socketFD
		s.init(42)
		assert.Equal(t, s.refCount(), 0)

		s.closeFunc(closeFD)
		assert.Equal(t, lastCloseFD, 42)
	})

	t.Run("release with zero ref count", func(t *testing.T) {
		var s socketFD
		s.init(21)

		fd := s.acquire()
		assert.Equal(t, fd, 21)
		assert.Equal(t, s.refCount(), 1)

		lastCloseFD = -1
		s.releaseFunc(fd, closeFD)
		assert.Equal(t, lastCloseFD, -1)
		assert.Equal(t, s.refCount(), 0)
	})

	t.Run("close with non zero ref count", func(t *testing.T) {
		var s socketFD
		s.init(10)

		fd0 := s.acquire()
		assert.Equal(t, fd0, 10)
		assert.Equal(t, s.refCount(), 1)

		fd1 := s.acquire()
		assert.Equal(t, fd1, 10)
		assert.Equal(t, s.refCount(), 2)

		lastCloseFD = -1
		s.closeFunc(closeFD)
		assert.Equal(t, lastCloseFD, -1)

		s.releaseFunc(fd0, closeFD)
		assert.Equal(t, lastCloseFD, -1)
		assert.Equal(t, s.refCount(), 1)

		s.releaseFunc(fd1, closeFD)
		assert.Equal(t, lastCloseFD, 10)
		assert.Equal(t, s.refCount(), 0)
	})
}
