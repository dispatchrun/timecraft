package network

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func (hostNamespace) Socket(family Family, socktype Socktype, protocol Protocol) (Socket, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	fd, err := ignoreEINTR2(func() (int, error) {
		return unix.Socket(int(family), int(socktype), int(protocol))
	})
	if err != nil {
		return nil, err
	}
	if err := setCloseOnExecAndNonBlocking(fd); err != nil {
		unix.Close(fd)
		return nil, err
	}
	return newHostSocket(fd, family, socktype), nil
}

func (s *hostSocket) Accept() (Socket, Sockaddr, error) {
	fd := s.acquire()
	if fd < 0 {
		return nil, nil, EBADF
	}
	defer s.release(fd)
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	conn, addr, err := ignoreEINTR3(func() (int, Sockaddr, error) {
		return unix.Accept(fd)
	})
	if err != nil {
		return nil, nil, err
	}
	if err := setCloseOnExecAndNonBlocking(conn); err != nil {
		unix.Close(conn)
		return nil, nil, err
	}
	return newHostSocket(conn, s.family, s.socktype), addr, nil
}

func setCloseOnExecAndNonBlocking(fd int) error {
	if _, err := ignoreEINTR2(func() (int, error) {
		return unix.FcntlInt(uintptr(fd), unix.F_SETFD, unix.O_CLOEXEC)
	}); err != nil {
		return err
	}
	if err := ignoreEINTR(func() error {
		return unix.SetNonblock(fd, true)
	}); err != nil {
		return err
	}
	return nil
}
