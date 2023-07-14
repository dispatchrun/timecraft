package network

import (
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
	return bind(fd, addr)
}

func (s *hostSocket) Listen(backlog int) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return listen(fd, backlog)
}

func (s *hostSocket) Connect(addr Sockaddr) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return connect(fd, addr)
}

func (s *hostSocket) Name() (Sockaddr, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.fd.release(fd)
	return getsockname(fd)
}

func (s *hostSocket) Peer() (Sockaddr, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.fd.release(fd)
	return getpeername(fd)
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
	return shutdown(fd, how)
}

func (s *hostSocket) SetOptInt(level, name, value int) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return setsockoptInt(fd, level, name, value)
}

func (s *hostSocket) GetOptInt(level, name int) (int, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.fd.release(fd)
	return getsockoptInt(fd, level, name)
}
