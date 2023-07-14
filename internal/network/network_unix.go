package network

import (
	"sync/atomic"

	"golang.org/x/sys/unix"
)

const (
	EADDRNOTAVAIL = unix.EADDRNOTAVAIL
	EAFNOSUPPORT  = unix.EAFNOSUPPORT
	EBADF         = unix.EBADF
	ECONNREFUSED  = unix.ECONNREFUSED
	ECONNRESET    = unix.ECONNRESET
	EHOSTUNREACH  = unix.EHOSTUNREACH
	EINVAL        = unix.EINVAL
	EINTR         = unix.EINTR
	EINPROGRESS   = unix.EINPROGRESS
	EISCONN       = unix.EISCONN
	ENETUNREACH   = unix.ENETUNREACH
	ENOPROTOOPT   = unix.ENOPROTOOPT
	ENOSYS        = unix.ENOSYS
	ENOTCONN      = unix.ENOTCONN
)

const (
	UNIX  Family = unix.AF_UNIX
	INET  Family = unix.AF_INET
	INET6 Family = unix.AF_INET6
)

const (
	STREAM Socktype = unix.SOCK_STREAM
	DGRAM  Socktype = unix.SOCK_DGRAM
)

const (
	TRUNC   = unix.MSG_TRUNC
	PEEK    = unix.MSG_PEEK
	WAITALL = unix.MSG_WAITALL
)

const (
	SHUTRD = unix.SHUT_RD
	SHUTWR = unix.SHUT_WR
)

type Sockaddr = unix.Sockaddr
type SockaddrInet4 = unix.SockaddrInet4
type SockaddrInet6 = unix.SockaddrInet6

type socketFD struct {
	state atomic.Uint64 // upper 32 bits: refCount, lower 32 bits: fd
}

func (s *socketFD) init(fd int) {
	s.state.Store(uint64(fd))
}

func (s *socketFD) load() int {
	return int(int32(s.state.Load()))
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

func (s *socketFD) release(fd int) {
	for {
		oldState := s.state.Load()
		refCount := (oldState >> 32) - 1
		newState := (oldState << 32) | (oldState & 0xFFFFFFFF)

		if s.state.CompareAndSwap(oldState, newState) {
			if int32(oldState) < 0 && refCount == 0 {
				unix.Close(fd)
			}
			break
		}
	}
}

func (s *socketFD) close() error {
	for {
		oldState := s.state.Load()
		refCount := oldState >> 32
		newState := oldState | 0xFFFFFFFF

		if s.state.CompareAndSwap(oldState, newState) {
			fd := int32(oldState)
			if fd < 0 {
				return EBADF
			}
			if refCount == 0 {
				return unix.Close(int(fd))
			}
			return nil
		}
	}
}
