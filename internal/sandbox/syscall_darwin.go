package sandbox

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	O_DSYNC = unix.O_SYNC
	O_RSYNC = unix.O_SYNC
)

const (
	openPathFlags = unix.O_DIRECTORY | unix.O_NOFOLLOW

	_PATH_MAX   = 1024
	_UTIME_NOW  = -1
	_UTIME_OMIT = -2
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

func prepareTimesAndAttrs(ts *[2]unix.Timespec) (attrs, size int, times [2]unix.Timespec) {
	const sizeOfTimespec = int(unsafe.Sizeof(times[0]))
	i := 0
	if ts[1].Nsec != _UTIME_OMIT {
		attrs |= unix.ATTR_CMN_MODTIME
		times[i] = ts[1]
		i++
	}
	if ts[0].Nsec != _UTIME_OMIT {
		attrs |= unix.ATTR_CMN_ACCTIME
		times[i] = ts[0]
		i++
	}
	return attrs, i * sizeOfTimespec, times
}

func futimens(fd int, ts *[2]unix.Timespec) error {
	attrs, size, times := prepareTimesAndAttrs(ts)
	attrlist := unix.Attrlist{
		Bitmapcount: unix.ATTR_BIT_MAP_COUNT,
		Commonattr:  uint32(attrs),
	}
	return ignoreEINTR(func() error {
		return fsetattrlist(fd, &attrlist, unsafe.Pointer(&times), size, 0)
	})
}

func fsetattrlist(fd int, attrlist *unix.Attrlist, attrbuf unsafe.Pointer, attrbufsize int, options uint32) error {
	_, _, e := unix.Syscall6(
		uintptr(unix.SYS_FSETATTRLIST),
		uintptr(fd),
		uintptr(unsafe.Pointer(attrlist)),
		uintptr(attrbuf),
		uintptr(attrbufsize),
		uintptr(options),
		uintptr(0),
	)
	if e != 0 {
		return e
	}
	return nil
}

func freadlink(fd int) (string, error) {
	const SYS_FREADLINK = 551
	buf := [_PATH_MAX + 1]byte{}
	n, _, e := syscall.Syscall(
		uintptr(SYS_FREADLINK),
		uintptr(fd),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(len(buf)),
	)
	if e != 0 {
		return "", e
	}
	if int(n) == len(buf) {
		return "", unix.ENAMETOOLONG
	}
	return string(buf[:n]), nil
}
