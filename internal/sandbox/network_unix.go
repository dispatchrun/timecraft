package sandbox

import (
	"os"
	"runtime"
	"time"

	"golang.org/x/sys/unix"
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

func setFileDeadline(f *os.File, rtimeout, wtimeout time.Duration) error {
	var now time.Time
	if rtimeout > 0 || wtimeout > 0 {
		now = time.Now()
	}
	if rtimeout > 0 {
		if err := f.SetReadDeadline(now.Add(rtimeout)); err != nil {
			return err
		}
	}
	if wtimeout > 0 {
		if err := f.SetWriteDeadline(now.Add(wtimeout)); err != nil {
			return err
		}
	}
	return nil
}

func handleSocketIOError(err error) error {
	if err != nil {
		if err == os.ErrDeadlineExceeded {
			err = EAGAIN
		}
	}
	return err
}
