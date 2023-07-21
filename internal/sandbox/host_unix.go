package sandbox

import (
	"os"
	"runtime"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type hostSocket struct {
	fd       socketFD
	family   Family
	socktype Socktype
	file     *os.File
	nonblock bool
	rtimeout time.Duration
	wtimeout time.Duration
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

func (s *hostSocket) Fd() uintptr {
	return uintptr(s.fd.load())
}

func (s *hostSocket) Close() error {
	s.fd.close()
	if s.file != nil {
		s.file.Close()
	}
	return nil
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

func (s *hostSocket) Accept() (Socket, Sockaddr, error) {
	var conn int
	var addr Sockaddr
	var err error

	if s.nonblock {
		fd := s.fd.acquire()
		if fd < 0 {
			return nil, nil, EBADF
		}
		defer s.fd.release(fd)
		conn, addr, err = accept(fd)
	} else {
		rawConn, err := s.syscallConn()
		if err != nil {
			return nil, nil, err
		}
		rawConnErr := rawConn.Read(func(fd uintptr) bool {
			conn, addr, err = accept(int(fd))
			if err != EAGAIN {
				return true
			}
			err = nil
			return false
		})
		if err == nil {
			err = rawConnErr
		}
	}

	if err != nil {
		return nil, nil, handleSocketIOError(err)
	}
	return newHostSocket(conn, s.family, s.socktype), addr, nil
}

func (s *hostSocket) Connect(addr Sockaddr) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)

	// In some cases, Linux allows sockets to be connected to addresses of a
	// different family (e.g. AF_INET datagram sockets connecting to AF_INET6
	// addresses). This is not portable, until we have a clear use case it is
	// wiser to disallow it, valid programs should use address families that
	// match the socket domain.
	if runtime.GOOS == "linux" {
		if s.family != SockaddrFamily(addr) {
			return EAFNOSUPPORT
		}
	}

	err := connect(fd, addr)
	if err != EINPROGRESS || s.nonblock {
		return err
	}

	rawConn, err := s.syscallConn()
	if err != nil {
		return err
	}

	rawConnErr := rawConn.Write(func(fd uintptr) bool {
		var value int
		value, err = getsockoptInt(int(fd), SOL_SOCKET, SO_ERROR)
		if err != nil {
			return true // done
		}
		switch unix.Errno(value) {
		case EINPROGRESS, EINTR:
			return false // continue
		case EISCONN:
			err = nil
			return true
		case unix.Errno(0):
			// The net poller can wake up spuriously. Check that we are
			// are really connected.
			_, err := getpeername(int(fd))
			return err == nil
		default:
			err = unix.Errno(value)
			return true
		}
	})
	if err == nil {
		err = rawConnErr
	}
	return err
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
	if s.nonblock {
		fd := s.fd.acquire()
		if fd < 0 {
			return -1, 0, nil, EBADF
		}
		defer s.fd.release(fd)
		return recvfrom(fd, iovs, flags)
	}
	var n, rflags int
	var addr Sockaddr
	rawConn, err := s.syscallConn()
	if err != nil {
		return -1, 0, nil, err
	}
	rawConnErr := rawConn.Read(func(fd uintptr) bool {
		n, rflags, addr, err = recvfrom(int(fd), iovs, flags)
		if err != EAGAIN {
			return true
		}
		err = nil
		return false
	})
	if err == nil {
		err = rawConnErr
	}
	return n, rflags, addr, handleSocketIOError(err)
}

func (s *hostSocket) SendTo(iovs [][]byte, addr Sockaddr, flags int) (int, error) {
	if s.nonblock {
		fd := s.fd.acquire()
		if fd < 0 {
			return -1, EBADF
		}
		defer s.fd.release(fd)
		return sendto(fd, iovs, addr, flags)
	}
	var n int
	rawConn, err := s.syscallConn()
	if err != nil {
		return -1, err
	}
	rawConnErr := rawConn.Write(func(fd uintptr) bool {
		n, err = sendto(int(fd), iovs, addr, flags)
		if err != EAGAIN {
			return true
		}
		err = nil
		return false
	})
	if err == nil {
		err = rawConnErr
	}
	return n, handleSocketIOError(err)
}

func (s *hostSocket) Shutdown(how int) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return shutdown(fd, how)
}

func (s *hostSocket) GetOptInt(level, name int) (int, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.fd.release(fd)
	return getsockoptInt(fd, level, name)
}

func (s *hostSocket) GetOptString(level, name int) (string, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return "", EBADF
	}
	defer s.fd.release(fd)
	return getsockoptString(fd, level, name)
}

func (s *hostSocket) GetOptTimeval(level, name int) (Timeval, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return Timeval{}, EBADF
	}
	defer s.fd.release(fd)

	switch level {
	case SOL_SOCKET:
		switch name {
		case SO_RCVTIMEO:
			return unix.NsecToTimeval(int64(s.rtimeout)), nil
		case SO_SNDTIMEO:
			return unix.NsecToTimeval(int64(s.wtimeout)), nil
		}
	}

	return getsockoptTimeval(fd, level, name)
}

func (s *hostSocket) SetOptInt(level, name, value int) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return setsockoptInt(fd, level, name, value)
}

func (s *hostSocket) SetOptString(level, name int, value string) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return setsockoptString(fd, level, name, value)
}

func (s *hostSocket) SetOptTimeval(level, name int, value Timeval) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)

	switch level {
	case SOL_SOCKET:
		switch name {
		case SO_RCVTIMEO:
			s.rtimeout = time.Duration(value.Nano())
			return nil
		case SO_SNDTIMEO:
			s.wtimeout = time.Duration(value.Nano())
			return nil
		}
	}

	return setsockoptTimeval(fd, level, name, value)
}

func (s *hostSocket) SetNonblock(nonblock bool) {
	s.nonblock = nonblock
}

func (s *hostSocket) IsNonblock() bool {
	return s.nonblock
}

func (s *hostSocket) syscallConn() (syscall.RawConn, error) {
	f, err := s.syscallFile()
	if err != nil {
		return nil, err
	}
	if err := setFileDeadline(f, s.rtimeout, s.wtimeout); err != nil {
		return nil, err
	}
	return f.SyscallConn()
}

func (s *hostSocket) syscallFile() (*os.File, error) {
	if s.file != nil {
		return s.file, nil
	}
	f := s.fd.file()
	if f == nil {
		return nil, EBADF
	}
	s.file = f
	return f, nil
}
