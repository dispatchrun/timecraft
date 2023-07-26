package sandbox

import (
	"syscall"

	"golang.org/x/sys/unix"
)

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
		closeTraceError(conn)
		return -1, nil, err
	}
	return conn, addr, nil
}

func socket(family, socktype, protocol int) (int, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	fd, err := ignoreEINTR2(func() (int, error) {
		return unix.Socket(family, socktype, protocol)
	})
	if err != nil {
		// Darwin gives EPROTOTYPE when the socket type and protocol do
		// not match, which differs from the Linux behavior which returns
		// EPROTONOSUPPORT. Since there is no real use case for dealing
		// with the error differently, and valid applications will not
		// invoke SockOpen with invalid parameters, we align on the Linux
		// behavior for simplicity.
		if err == unix.EPROTOTYPE {
			err = unix.EPROTONOSUPPORT
		}
		return -1, err
	}
	if err := setCloseOnExecAndNonBlocking(fd); err != nil {
		closeTraceError(fd)
		return -1, err
	}
	return fd, nil
}

func socketpair(family, socktype, protocol int) ([2]int, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()

	fds, err := ignoreEINTR2(func() ([2]int, error) {
		return unix.Socketpair(family, socktype, protocol)
	})
	if err != nil {
		return fds, err
	}
	if err := setCloseOnExecAndNonBlocking(fds[0]); err != nil {
		closePair(&fds)
		return fds, err
	}
	if err := setCloseOnExecAndNonBlocking(fds[1]); err != nil {
		closePair(&fds)
		return fds, err
	}
	return fds, nil
}

func pipe(fds *[2]int) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	if err := unix.Pipe(fds[:]); err != nil {
		return err
	}
	if err := setCloseOnExecAndNonBlocking(fds[0]); err != nil {
		closePair(fds)
		return err
	}
	if err := setCloseOnExecAndNonBlocking(fds[1]); err != nil {
		closePair(fds)
		return err
	}
	return nil
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

func fdatasync(fd int) error {
	return unix.Fsync(fd)
}
