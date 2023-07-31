package sandbox

import (
	"fmt"
	"os"
	"runtime"
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
	EEXIST          = unix.EEXIST
	EHOSTUNREACH    = unix.EHOSTUNREACH
	EINVAL          = unix.EINVAL
	EINTR           = unix.EINTR
	EINPROGRESS     = unix.EINPROGRESS
	EISCONN         = unix.EISCONN
	EISDIR          = unix.EISDIR
	ELOOP           = unix.ELOOP
	ENAMETOOLONG    = unix.ENAMETOOLONG
	ENETUNREACH     = unix.ENETUNREACH
	ENOENT          = unix.ENOENT
	ENOPROTOOPT     = unix.ENOPROTOOPT
	ENOSYS          = unix.ENOSYS
	ENOTCONN        = unix.ENOTCONN
	ENOTDIR         = unix.ENOTDIR
	ENOTEMPTY       = unix.ENOTEMPTY
	EOPNOTSUPP      = unix.EOPNOTSUPP
	EPERM           = unix.EPERM
	EPROTONOSUPPORT = unix.EPROTONOSUPPORT
	EPROTOTYPE      = unix.EPROTOTYPE
	ETIMEDOUT       = unix.ETIMEDOUT
	EXDEV           = unix.EXDEV
)

const (
	O_RDONLY    = unix.O_RDONLY
	O_WRONLY    = unix.O_WRONLY
	O_RDWR      = unix.O_RDWR
	O_APPEND    = unix.O_APPEND
	O_CREAT     = unix.O_CREAT
	O_EXCL      = unix.O_EXCL
	O_SYNC      = unix.O_SYNC
	O_TRUNC     = unix.O_TRUNC
	O_DIRECTORY = unix.O_DIRECTORY
	O_NOFOLLOW  = unix.O_NOFOLLOW
)

const (
	AT_SYMLINK_NOFOLLOW = unix.AT_SYMLINK_NOFOLLOW
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
type SockaddrUnix = unix.SockaddrUnix
type Timeval = unix.Timeval

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

func closePair(fds *[2]int) {
	if fds[0] >= 0 {
		closeTraceError(fds[0])
	}
	if fds[1] >= 0 {
		closeTraceError(fds[1])
	}
	fds[0] = -1
	fds[1] = -1
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

func bind(fd int, addr Sockaddr) error {
	return ignoreEINTR(func() error { return unix.Bind(fd, addr) })
}

func listen(fd, backlog int) error {
	return ignoreEINTR(func() error { return unix.Listen(fd, backlog) })
}

func connect(fd int, addr Sockaddr) error {
	err := ignoreEINTR(func() error { return unix.Connect(fd, addr) })
	switch err {
	// Linux gives EINVAL only when trying to connect to an ipv4 address
	// from an ipv6 address. Darwin does not seem to return EINVAL but it
	// documents that it might if the address family does not match, so we
	// normalize the the error value here.
	case EINVAL:
		err = EAFNOSUPPORT
	// Darwin gives EOPNOTSUPP when trying to connect a socket that is
	// already connected or already listening. Align on the Linux behavior
	// here and convert the error to EISCONN.
	case EOPNOTSUPP:
		err = EISCONN
	}
	return err
}

func shutdown(fd, how int) error {
	// Linux allows calling shutdown(2) on listening sockets, but not Darwin.
	// To provide a portable behavior we align on the POSIX behavior which says
	// that shutting down non-connected sockets must return ENOTCONN.
	//
	// Note that this may cause issues in the future if applications need a way
	// to break out of a blocking accept(2) call. We could relax this limitation
	// down the line, tho keep in mind that applications may be better served by
	// not relying on system-specific behaviors and should use synchronization
	// mechanisms is user-space to maximize portability.
	//
	// For more context see: https://bugzilla.kernel.org/show_bug.cgi?id=106241
	if runtime.GOOS == "linux" {
		v, err := ignoreEINTR2(func() (int, error) {
			return unix.GetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_ACCEPTCONN)
		})
		if err != nil {
			return err
		}
		if v != 0 {
			return ENOTCONN
		}
	}
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

func setsockoptInt(fd, level, name, value int) error {
	switch level {
	case unix.SOL_SOCKET:
		switch name {
		case unix.SO_RCVBUF, unix.SO_SNDBUF:
			// Treat setting negative buffer sizes as a special, invalid case to
			// ensure portability across operating systems.
			if value < 0 {
				return EINVAL
			}
			// Linux allows setting the socket buffer size to zero, but darwin
			// does not, so we hardcode the limit for OSX.
			if runtime.GOOS == "darwin" {
				const minBufferSize = 4 * 1024
				const maxBufferSize = 4 * 1024 * 1024
				switch {
				case value < minBufferSize:
					value = minBufferSize
				case value > maxBufferSize:
					value = maxBufferSize
				}
			}
		}
	}
	return ignoreEINTR(func() error { return unix.SetsockoptInt(fd, level, name, value) })
}

func recvfrom(fd int, iovs [][]byte, flags int) (n, rflags int, addr Sockaddr, err error) {
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

func recvmsg(fd int, msg, oob []byte, flags int) (n, oobn, rflags int, addr Sockaddr, err error) {
	// TOOD: remove the heap allocation for the receive address by
	// implementing recvmsg and using the stack-allocated socket address
	// buffer.
	for {
		n, oobn, rflags, addr, err := unix.Recvmsg(fd, msg, oob, flags)
		if err == EINTR {
			if n == 0 {
				continue
			}
			err = nil
		}
		return n, oobn, rflags, addr, err
	}
}

func sendto(fd int, iovs [][]byte, addr Sockaddr, flags int) (int, error) {
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

func sendmsg(fd int, msg, oob []byte, addr Sockaddr, flags int) error {
	return ignoreEINTR(func() error { return unix.Sendmsg(fd, msg, oob, addr, flags) })
}

func fstat(fd int, stat *unix.Stat_t) error {
	return ignoreEINTR(func() error { return unix.Fstat(fd, stat) })
}

func fstatat(dirfd int, path string, stat *unix.Stat_t, flags int) error {
	return ignoreEINTR(func() error { return unix.Fstatat(dirfd, path, stat, flags) })
}

func readlinkat(dirfd int, path string) (string, error) {
	buf := make([]byte, 256)
	for {
		n, err := ignoreEINTR2(func() (int, error) { return unix.Readlinkat(dirfd, path, buf) })
		if err != nil {
			return "", err
		}
		if n < len(buf) {
			return string(buf[:n]), nil
		}
		if len(buf) >= _PATH_MAX {
			return "", ENAMETOOLONG
		}
		buf = make([]byte, 2*len(buf))
	}
}

func utimensat(dirfd int, path string, ts *[2]unix.Timespec, flags int) error {
	return ignoreEINTR(func() error { return unix.UtimesNanoAt(dirfd, path, ts[:], flags) })
}

func mkdirat(dirfd int, path string, mode uint32) error {
	return ignoreEINTR(func() error { return unix.Mkdirat(dirfd, path, mode) })
}

func renameat(olddirfd int, oldpath string, newdirfd int, newpath string) error {
	return ignoreEINTR(func() error { return unix.Renameat(olddirfd, oldpath, newdirfd, newpath) })
}

func linkat(olddirfd int, oldpath string, newdirfd int, newpath string, flags int) error {
	return ignoreEINTR(func() error { return unix.Linkat(olddirfd, oldpath, newdirfd, newpath, flags) })
}

func symlinkat(target string, dirfd int, path string) error {
	return ignoreEINTR(func() error { return unix.Symlinkat(target, dirfd, path) })
}

func unlinkat(dirfd int, path string, flags int) error {
	return ignoreEINTR(func() error { return unix.Unlinkat(dirfd, path, flags) })
}
