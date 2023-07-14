package network

import (
	"time"

	"golang.org/x/sys/unix"
)

type hostSocket struct {
	fd       socketFD
	family   Family
	socktype Socktype
}

func newHostSocket(fd int, family Family, socktype Socktype) *hostSocket {
	s := &hostSocket{family: family, socktype: socktype}
	s.fd.init(fd)
	return s
}

func (s *hostSocket) Family() Family {
	return s.family
}

func (s *hostSocket) Type() Socktype {
	return s.socktype
}

func (s *hostSocket) Fd() int {
	return s.fd.load()
}

func (s *hostSocket) Close() error {
	return s.fd.close()
}

func (s *hostSocket) Bind(addr Sockaddr) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return ignoreEINTR(func() error { return unix.Bind(fd, addr) })
}

func (s *hostSocket) Listen(backlog int) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return ignoreEINTR(func() error { return unix.Listen(fd, backlog) })
}

func (s *hostSocket) Connect(addr Sockaddr) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return ignoreEINTR(func() error { return unix.Connect(fd, addr) })
}

func (s *hostSocket) Name() (Sockaddr, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.fd.release(fd)
	return ignoreEINTR2(func() (Sockaddr, error) { return unix.Getsockname(fd) })
}

func (s *hostSocket) Peer() (Sockaddr, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.fd.release(fd)
	return ignoreEINTR2(func() (Sockaddr, error) { return unix.Getpeername(fd) })
}

func (s *hostSocket) RecvFrom(iovs [][]byte, flags int) (int, int, Sockaddr, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return -1, 0, nil, EBADF
	}
	defer s.fd.release(fd)
	// TODO: remove the heap allocation that happens for the socket address by
	// implementing recvfrom(2) and using a cached socket address for connected
	// sockets.
	for {
		n, _, rflags, addr, err := unix.RecvmsgBuffers(fd, iovs, nil, flags)
		if err == EINTR {
			if n == 0 {
				continue
			}
			err = nil
		}
		return n, rflags, addr, err
	}
}

func (s *hostSocket) SendTo(iovs [][]byte, addr Sockaddr, flags int) (int, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.fd.release(fd)
	for {
		n, err := unix.SendmsgBuffers(fd, iovs, nil, addr, flags)
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
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return ignoreEINTR(func() error {
		return unix.Shutdown(fd, how)
	})
}

func (s *hostSocket) SetOptInt(level, name, value int) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return ignoreEINTR(func() error { return unix.SetsockoptInt(fd, level, name, value) })
}

func (s *hostSocket) GetOptInt(level, name int) (int, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.fd.release(fd)
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
