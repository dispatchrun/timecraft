package network

import (
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

type hostSocket struct {
	state    atomic.Uint64 // upper 32 bits: refCount, lower 32 bits: fd
	family   Family
	socktype Socktype
}

func newHostSocket(fd int, family Family, socktype Socktype) *hostSocket {
	s := &hostSocket{family: family, socktype: socktype}
	s.state.Store(uint64(fd))
	return s
}

func (s *hostSocket) acquire() int {
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

func (s *hostSocket) release(fd int) {
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

func (s *hostSocket) Family() Family {
	return s.family
}

func (s *hostSocket) Type() Socktype {
	return s.socktype
}

func (s *hostSocket) Fd() int {
	return int(int32(s.state.Load()))
}

func (s *hostSocket) Close() error {
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

func (s *hostSocket) Bind(addr Sockaddr) error {
	fd := s.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.release(fd)
	return ignoreEINTR(func() error { return unix.Bind(fd, addr) })
}

func (s *hostSocket) Listen(backlog int) error {
	fd := s.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.release(fd)
	return ignoreEINTR(func() error { return unix.Listen(fd, backlog) })
}

func (s *hostSocket) Connect(addr Sockaddr) error {
	fd := s.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.release(fd)
	return ignoreEINTR(func() error { return unix.Connect(fd, addr) })
}

func (s *hostSocket) Name() (Sockaddr, error) {
	fd := s.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.release(fd)
	return ignoreEINTR2(func() (Sockaddr, error) { return unix.Getsockname(fd) })
}

func (s *hostSocket) Peer() (Sockaddr, error) {
	fd := s.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.release(fd)
	return ignoreEINTR2(func() (Sockaddr, error) { return unix.Getpeername(fd) })
}

func (s *hostSocket) RecvFrom(iovs [][]byte, oob []byte, flags int) (int, int, int, Sockaddr, error) {
	fd := s.acquire()
	if fd < 0 {
		return -1, 0, 0, nil, EBADF
	}
	defer s.release(fd)
	// TODO: remove the heap allocation that happens for the socket address by
	// implementing recvfrom(2) and using a cached socket address for connected
	// sockets.
	for {
		n, oobn, rflags, addr, err := unix.RecvmsgBuffers(fd, iovs, oob, flags)
		if err == EINTR {
			if n == 0 {
				continue
			}
			err = nil
		}
		return n, oobn, rflags, addr, err
	}
}

func (s *hostSocket) SendTo(iovs [][]byte, oob []byte, addr Sockaddr, flags int) (int, error) {
	fd := s.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.release(fd)
	for {
		n, err := unix.SendmsgBuffers(fd, iovs, oob, addr, flags)
		if err == EINTR {
			if n == 0 {
				continue
			}
			err = nil
		}
		return n, err
	}
}

func (s *hostSocket) Shutdown(how int) error {
	fd := s.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.release(fd)
	return ignoreEINTR(func() error {
		return unix.Shutdown(fd, how)
	})
}

func (s *hostSocket) SetOption(level, name, value int) error {
	fd := s.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.release(fd)
	return ignoreEINTR(func() error { return unix.SetsockoptInt(fd, level, name, value) })
}

func (s *hostSocket) GetOption(level, name int) (int, error) {
	fd := s.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.release(fd)
	return ignoreEINTR2(func() (int, error) { return unix.GetsockoptInt(fd, level, name) })
}

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
