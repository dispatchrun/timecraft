package network

import (
	"sync/atomic"
)

// socketFD is used to manage the lifecycle of socket file descriptors;
// it allows multiple goroutines to share ownership of the socket while
// coordinating to close the file descriptor via an atomic reference count.
//
// Goroutines must call acquire to access the file descriptor; if they get a
// negative number, it indicates that the socket was already closed and the
// method should usually return EBADF.
//
// After acquiring a valid file descriptor, the goroutine is responsible for
// calling release with the same fd number that was returned by acquire. The
// release may cause the file descriptor to be closed if the close method was
// called in between and releasing the fd causes the reference count to reach
// zero.
//
// The close method detaches the file descriptor from the socketFD, but it only
// closes it if the reference count is zero (no other goroutines was sharing
// ownership). After closing the socketFD, all future calls to acquire return a
// negative number, preventing other goroutines from acquiring ownership of the
// file descriptor and guaranteeing that it will eventually be closed.
type socketFD struct {
	state atomic.Uint64 // upper 32 bits: refCount, lower 32 bits: fd
}

func (s *socketFD) init(fd int) {
	s.state.Store(uint64(fd & 0xFFFFFFFF))
}

func (s *socketFD) load() int {
	return int(int32(s.state.Load()))
}

func (s *socketFD) refCount() int {
	return int(s.state.Load() >> 32)
}

func (s *socketFD) acquire() int {
	for {
		oldState := s.state.Load()
		refCount := (oldState >> 32) + 1
		newState := (refCount << 32) | (oldState & 0xFFFFFFFF)

		fd := int32(oldState)
		if fd < 0 {
			return -1
		}
		if s.state.CompareAndSwap(oldState, newState) {
			return int(fd)
		}
	}
}

func (s *socketFD) releaseFunc(fd int, closeFD func(int)) {
	for {
		oldState := s.state.Load()
		refCount := (oldState >> 32) - 1
		newState := (refCount << 32) | (oldState & 0xFFFFFFFF)

		if s.state.CompareAndSwap(oldState, newState) {
			if int32(oldState) < 0 && refCount == 0 {
				closeFD(fd)
			}
			break
		}
	}
}

func (s *socketFD) closeFunc(closeFD func(int)) {
	for {
		oldState := s.state.Load()
		refCount := oldState >> 32
		newState := oldState | 0xFFFFFFFF

		fd := int32(oldState)
		if fd < 0 {
			break
		}
		if s.state.CompareAndSwap(oldState, newState) {
			if refCount == 0 {
				closeFD(int(fd))
			}
			break
		}
	}
}
