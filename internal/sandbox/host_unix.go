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
	connect  bool
	listen   bool
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

func (s *hostSocket) File() *os.File {
	if s.file != nil {
		return s.file
	}
	fd := s.fd.acquire()
	if fd < 0 {
		return nil
	}
	defer s.fd.release(fd)
	fileFd, err := dup(fd)
	if err != nil {
		return nil
	}
	f := os.NewFile(uintptr(fileFd), "")
	s.file = f
	return f
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
	if err := listen(fd, backlog); err != nil {
		return err
	}
	s.listen = true
	return nil
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

	s.connect = true
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
		value, err = getsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_ERROR)
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
	if s.connect && addr != nil {
		return 0, EISCONN
	}
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

func (s *hostSocket) Error() error {
	v, err := s.getOptInt(unix.SOL_SOCKET, unix.SO_ERROR)
	if err != nil {
		return err
	}
	return unix.Errno(v)
}

func (s *hostSocket) IsListening() (bool, error) {
	return s.listen, nil
}

func (s *hostSocket) IsNonBlock() (bool, error) {
	return s.nonblock, nil
}

func (s *hostSocket) RecvBuffer() (int, error) {
	return s.getOptInt(unix.SOL_SOCKET, unix.SO_RCVBUF)
}

func (s *hostSocket) SendBuffer() (int, error) {
	return s.getOptInt(unix.SOL_SOCKET, unix.SO_SNDBUF)
}

func (s *hostSocket) RecvTimeout() (time.Duration, error) {
	return s.rtimeout, nil
}

func (s *hostSocket) SendTimeout() (time.Duration, error) {
	return s.wtimeout, nil
}

func (s *hostSocket) TCPNoDelay() (bool, error) {
	return s.getOptBool(unix.IPPROTO_TCP, unix.TCP_NODELAY)
}

func (s *hostSocket) SetNonBlock(nonblock bool) error {
	s.nonblock = nonblock
	return nil
}

func (s *hostSocket) SetRecvBuffer(size int) error {
	return s.setOptInt(unix.SOL_SOCKET, unix.SO_RCVBUF, size)
}

func (s *hostSocket) SetSendBuffer(size int) error {
	return s.setOptInt(unix.SOL_SOCKET, unix.SO_SNDBUF, size)
}

func (s *hostSocket) SetRecvTimeout(timeout time.Duration) error {
	s.rtimeout = timeout
	return nil
}

func (s *hostSocket) SetSendTimeout(timeout time.Duration) error {
	s.wtimeout = timeout
	return nil
}

func (s *hostSocket) SetTCPNoDelay(nodelay bool) error {
	return s.setOptBool(unix.IPPROTO_TCP, unix.TCP_NODELAY, nodelay)
}

func (s *hostSocket) SetTLSServerName(serverName string) error {
	return EOPNOTSUPP
}

func (s *hostSocket) getOptBool(level, name int) (bool, error) {
	v, err := s.getOptInt(level, name)
	if err != nil {
		return false, err
	}
	return v != 0, nil
}

func (s *hostSocket) getOptInt(level, name int) (int, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.fd.release(fd)
	return getsockoptInt(fd, level, name)
}

func (s *hostSocket) setOptBool(level, name int, value bool) error {
	intValue := 0
	if value {
		intValue = 1
	}
	return s.setOptInt(level, name, intValue)
}

func (s *hostSocket) setOptInt(level, name, value int) error {
	fd := s.fd.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd.release(fd)
	return setsockoptInt(fd, level, name, value)
}

func (s *hostSocket) syscallConn() (syscall.RawConn, error) {
	f := s.File()
	if f == nil {
		return nil, EBADF
	}
	if err := setFileDeadline(f, s.rtimeout, s.wtimeout); err != nil {
		return nil, err
	}
	return f.SyscallConn()
}
