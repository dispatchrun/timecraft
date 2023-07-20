package network

import (
	"time"

	"golang.org/x/sys/unix"
)

const (
	EADDRNOTAVAIL = unix.EADDRNOTAVAIL
	EAFNOSUPPORT  = unix.EAFNOSUPPORT
	EBADF         = unix.EBADF
	ECONNABORTED  = unix.ECONNABORTED
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

func getsockoptString(fd, level, name int) (string, error) {
	return ignoreEINTR2(func() (string, error) { return unix.GetsockoptString(fd, level, name) })
}

func setsockoptInt(fd, level, name, value int) error {
	return ignoreEINTR(func() error { return unix.SetsockoptInt(fd, level, name, value) })
}

func setsockoptString(fd, level, name int, value string) error {
	return ignoreEINTR(func() error { return unix.SetsockoptString(fd, level, name, value) })
}
