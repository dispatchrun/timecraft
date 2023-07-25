package sandbox

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"sync/atomic"
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
	return ignoreEINTR(func() error {
		return unix.Sendmsg(fd, msg, oob, addr, flags)
	})
}

// fdRef is used to manage the lifecycle of socket file descriptors;
// it allows multiple goroutines to share ownership of the socket while
// coordinating to close the file descriptor via an atomic reference count.
//
// Goroutines must call acquire to access the file descriptor; if they get a
// negative number, it indicates that the socket was already closed and the
// method should usually return EBADF.
//
// After acquiring a valid file descriptor, the goroutine is responsible for
// calling release with the same fd number that was returned by acquire. The
// release may cause the file descriptor to be closed if the close method was
// called in between and releasing the fd causes the reference count to reach
// zero.
//
// The close method detaches the file descriptor from the fdRef, but it only
// closes it if the reference count is zero (no other goroutines was sharing
// ownership). After closing the fdRef, all future calls to acquire return a
// negative number, preventing other goroutines from acquiring ownership of the
// file descriptor and guaranteeing that it will eventually be closed.
type fdRef struct {
	state atomic.Uint64 // upper 32 bits: refCount, lower 32 bits: fd
}

func (f *fdRef) init(fd int) {
	f.state.Store(uint64(fd & 0xFFFFFFFF))
}

func (f *fdRef) load() int {
	return int(int32(f.state.Load()))
}

func (f *fdRef) refCount() int {
	return int(f.state.Load() >> 32)
}

func (f *fdRef) acquire() int {
	for {
		oldState := f.state.Load()
		refCount := (oldState >> 32) + 1
		newState := (refCount << 32) | (oldState & 0xFFFFFFFF)

		fd := int32(oldState)
		if fd < 0 {
			return -1
		}
		if f.state.CompareAndSwap(oldState, newState) {
			return int(fd)
		}
	}
}

func (f *fdRef) releaseFunc(fd int, closeFD func(int)) {
	for {
		oldState := f.state.Load()
		refCount := (oldState >> 32) - 1
		newState := (refCount << 32) | (oldState & 0xFFFFFFFF)

		if f.state.CompareAndSwap(oldState, newState) {
			if int32(oldState) < 0 && refCount == 0 {
				closeFD(fd)
			}
			break
		}
	}
}

func (f *fdRef) closeFunc(closeFD func(int)) {
	for {
		oldState := f.state.Load()
		refCount := oldState >> 32
		newState := oldState | 0xFFFFFFFF

		fd := int32(oldState)
		if fd < 0 {
			break
		}
		if f.state.CompareAndSwap(oldState, newState) {
			if refCount == 0 {
				closeFD(int(fd))
			}
			break
		}
	}
}
