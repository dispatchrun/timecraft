package sandbox

import (
	"fmt"
	"os"
	"runtime/debug"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	EADDRNOTAVAIL   = unix.EADDRNOTAVAIL
	EAFNOSUPPORT    = unix.EAFNOSUPPORT
	EAGAIN          = unix.EAGAIN
	EBADF           = unix.EBADF
	ECONNABORTED    = unix.ECONNABORTED
	ECONNREFUSED    = unix.ECONNREFUSED
	ECONNRESET      = unix.ECONNRESET
	EHOSTUNREACH    = unix.EHOSTUNREACH
	EINVAL          = unix.EINVAL
	EINTR           = unix.EINTR
	EINPROGRESS     = unix.EINPROGRESS
	EISCONN         = unix.EISCONN
	ENETUNREACH     = unix.ENETUNREACH
	ENOPROTOOPT     = unix.ENOPROTOOPT
	ENOSYS          = unix.ENOSYS
	ENOTCONN        = unix.ENOTCONN
	EOPNOTSUPP      = unix.EOPNOTSUPP
	EPROTONOSUPPORT = unix.EPROTONOSUPPORT
	EPROTOTYPE      = unix.EPROTOTYPE
	ETIMEDOUT       = unix.ETIMEDOUT
)

// This function is used to automatically retry syscalls when they return EINTR
// due to having handled a signal instead of executing.
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

func dup(oldfd int) (int, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()
	newfd, err := ignoreEINTR2(func() (int, error) {
		return unix.Dup(oldfd)
	})
	if err != nil {
		return -1, err
	}
	unix.CloseOnExec(newfd)
	return newfd, nil
}

func closePipe(fds *[2]int) {
	if fds[0] >= 0 {
		closeTraceError(fds[0])
	}
	if fds[1] >= 0 {
		closeTraceError(fds[1])
	}
}

func closeTraceError(fd int) {
	if err := unix.Close(fd); err != nil {
		fmt.Fprintf(os.Stderr, "close(%d) => %s\n", fd, err)
		debug.PrintStack()
	}
}

func setNonblock(fd uintptr, nonblock bool) {
	if err := unix.SetNonblock(int(fd), nonblock); err != nil {
		fmt.Fprintf(os.Stderr, "setNonblock(%d,%t) => %s\n", fd, nonblock, err)
		debug.PrintStack()
	}
}
