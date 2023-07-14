package network

import (
	"sync/atomic"
)

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

		if int32(oldState) < 0 {
			return -1
		}
		if s.state.CompareAndSwap(oldState, newState) {
			return int(int32(oldState)) // int32->int for sign extension
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

		if s.state.CompareAndSwap(oldState, newState) {
			if fd := int32(oldState); fd >= 0 && refCount == 0 {
				closeFD(int(fd))
			}
			break
		}
	}
}
