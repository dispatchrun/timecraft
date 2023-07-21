package sandbox

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

func accept(fd int) (int, Sockaddr, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	conn, addr, err := ignoreEINTR3(func() (int, Sockaddr, error) {
		return unix.Accept(fd)
	})
	if err != nil {
		return -1, nil, err
	}
	if err := setCloseOnExecAndNonBlocking(conn); err != nil {
		unix.Close(conn)
		return -1, nil, err
	}
	return conn, addr, nil
}
