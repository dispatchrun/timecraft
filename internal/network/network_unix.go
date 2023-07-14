package network

import (
	"sync/atomic"
	"time"

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

// This function is used to automtically retry syscalls when they return EINTR
// due to having handled a signal instead of executing. Despite defininig a
// EINTR constant and having proc_raise to trigger signals from the guest, WASI
// does not provide any mechanism for handling signals so masking those errors
// seems like a safer approach to ensure that guest applications will work the
// same regardless of the compiler being used.
func ignoreEINTR(f func() error) error {
	for {
		if err := f(); err != EINTR {
			return err
		}
	}
}

func ignoreEINTR2[F func() (R, error), R any](f F) (R, error) {
	for {
		v, err := f()
		if err != EINTR {
			return v, err
		}
	}
}

func ignoreEINTR3[F func() (R1, R2, error), R1, R2 any](f F) (R1, R2, error) {
	for {
		v1, v2, err := f()
		if err != EINTR {
			return v1, v2, err
		}
	}
}

func WaitReadyRead(socket Socket, timeout time.Duration) error {
	return wait(socket, unix.POLLIN, timeout)
}

func WaitReadyWrite(socket Socket, timeout time.Duration) error {
	return wait(socket, unix.POLLOUT, timeout)
}

func wait(socket Socket, events int16, timeout time.Duration) error {
	tms := int(timeout / time.Millisecond)
	pfd := []unix.PollFd{{
		Fd:     int32(socket.Fd()),
		Events: events,
	}}
	return ignoreEINTR(func() error {
		_, err := unix.Poll(pfd, tms)
		return err
	})
}

func bind(fd int, addr Sockaddr) error {
	return ignoreEINTR(func() error { return unix.Bind(fd, addr) })
}

func listen(fd, backlog int) error {
	return ignoreEINTR(func() error { return unix.Listen(fd, backlog) })
}

func connect(fd int, addr Sockaddr) error {
	return ignoreEINTR(func() error { return unix.Connect(fd, addr) })
}

func shutdown(fd, how int) error {
	return ignoreEINTR(func() error { return unix.Shutdown(fd, how) })
}

func getsockname(fd int) (Sockaddr, error) {
	return ignoreEINTR2(func() (Sockaddr, error) { return unix.Getsockname(fd) })
}

func getpeername(fd int) (Sockaddr, error) {
	return ignoreEINTR2(func() (Sockaddr, error) { return unix.Getpeername(fd) })
}

func getsockoptInt(fd, level, name int) (int, error) {
	return ignoreEINTR2(func() (int, error) { return unix.GetsockoptInt(fd, level, name) })
}

func setsockoptInt(fd, level, name, value int) error {
	return ignoreEINTR(func() error { return unix.SetsockoptInt(fd, level, name, value) })
}

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
