package sandbox

import (
	"syscall"
	"unsafe"

	"github.com/stealthrocket/wasi-go"
	"golang.org/x/sys/unix"
)

const sizeOfDirent = 21

type dirent struct {
	ino    uint64
	off    uint64
	reclen uint16
	namlen uint16
	typ    uint8
}

func makeDirent(typ uint8, ino, off uint64, name string) dirent {
	return dirent{
		ino:    ino,
		off:    off,
		reclen: sizeOfDirent + uint16(len(name)) + 1,
		namlen: uint16(len(name)),
		typ:    typ,
	}
}

const (
	O_DSYNC OpenFlags = unix.O_SYNC
	O_RSYNC OpenFlags = unix.O_SYNC
)

const (
	RENAME_EXCHANGE  RenameFlags = 1 << 1
	RENAME_NOREPLACE RenameFlags = 1 << 2
)

const (
	openPathFlags = unix.O_DIRECTORY | unix.O_NOFOLLOW

	PATH_MAX   = 1024
	UTIME_NOW  = -1
	UTIME_OMIT = -2
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

func fallocate(fd int, offset, length int64) error {
	var stat unix.Stat_t
	if err := ignoreEINTR(func() error {
		return unix.Fstat(fd, &stat)
	}); err != nil {
		return err
	}
	if offset != stat.Size {
		return wasi.ENOSYS
	}
	err := ignoreEINTR(func() error {
		return unix.FcntlFstore(uintptr(fd), unix.F_PREALLOCATE, &unix.Fstore_t{
			Flags:   unix.F_ALLOCATEALL | unix.F_ALLOCATECONTIG,
			Posmode: unix.F_PEOFPOSMODE,
			Offset:  0,
			Length:  length,
		})
	})
	if err != nil {
		return err
	}
	return ignoreEINTR(func() error {
		return unix.Ftruncate(fd, stat.Size+length)
	})
}

func fdatasync(fd int) error {
	return ignoreEINTR(func() error {
		_, _, errno := syscall.Syscall(syscall.SYS_FDATASYNC, uintptr(fd), 0, 0)
		if errno != 0 {
			return errno
		}
		return nil
	})
}

func fsync(fd int) error {
	// See https://twitter.com/TigerBeetleDB/status/1422854887113732097
	_, err := unix.FcntlInt(uintptr(fd), unix.F_FULLFSYNC, 0)
	return err
}

func lseek(fd int, offset int64, whence int) (int64, error) {
	// Note: there is an issue with unix.Seek where it returns random error
	// values for delta >= 2^32-1; syscall.Seek does not appear to suffer from
	// this problem, nor does using unix.Syscall directly.
	//
	// The standard syscall package uses a special syscallX function to call
	// lseek, which x/sys/unix does not, here is the reason (copied from
	// src/runtime/sys_darwin.go):
	//
	//  The X versions of syscall expect the libc call to return a 64-bit result.
	//  Otherwise (the non-X version) expects a 32-bit result.
	//  This distinction is required because an error is indicated by returning -1,
	//  and we need to know whether to check 32 or 64 bits of the result.
	//  (Some libc functions that return 32 bits put junk in the upper 32 bits of AX.)
	//
	// return unix.Seek(f.FD, int64(delta), sysWhence)
	return syscall.Seek(fd, offset, whence)
}

func prepareTimesAndAttrs(ts *[2]unix.Timespec) (attrs, size int, times [2]unix.Timespec) {
	const sizeOfTimespec = int(unsafe.Sizeof(times[0]))
	i := 0
	if ts[1].Nsec != UTIME_OMIT {
		attrs |= unix.ATTR_CMN_MODTIME
		times[i] = ts[1]
		i++
	}
	if ts[0].Nsec != UTIME_OMIT {
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
	_, _, err := syscall.Syscall6(
		uintptr(syscall.SYS_FSETATTRLIST),
		uintptr(fd),
		uintptr(unsafe.Pointer(attrlist)),
		uintptr(attrbuf),
		uintptr(attrbufsize),
		uintptr(options),
		uintptr(0),
	)
	if err != 0 {
		return err
	}
	return nil
}

func freadlink(fd int, buf []byte) (int, error) {
	const SYS_FREADLINK = 551
	n, _, err := syscall.Syscall(
		uintptr(SYS_FREADLINK),
		uintptr(fd),
		uintptr(unsafe.Pointer(unsafe.SliceData(buf))),
		uintptr(len(buf)),
	)
	if err != 0 {
		return int(n), err
	}
	return int(n), nil
}

func read(fd int, buf []byte) (int, error) {
	return handleEINTR(func() (int, error) { return unix.Read(fd, buf) })
}

func write(fd int, buf []byte) (int, error) {
	return handleEINTR(func() (int, error) { return unix.Write(fd, buf) })
}

func pread(fd int, buf []byte, off int64) (int, error) {
	return handleEINTR(func() (int, error) { return unix.Pread(fd, buf, off) })
}

func pwrite(fd int, buf []byte, off int64) (int, error) {
	return handleEINTR(func() (int, error) { return unix.Pwrite(fd, buf, off) })
}

func readv(fd int, iovs [][]byte) (int, error) {
	rn := 0
	for _, iov := range iovs {
		n, err := read(fd, iov)
		if n > 0 {
			rn += n
		}
		if err != nil {
			return rn, err
		}
		if n < len(iov) {
			break
		}
	}
	return rn, nil
}

func writev(fd int, iovs [][]byte) (int, error) {
	wn := 0
	for _, iov := range iovs {
		n, err := write(fd, iov)
		if n > 0 {
			wn += n
		}
		if err != nil {
			return wn, err
		}
		if n < len(iov) {
			break
		}
	}
	return wn, nil
}

func preadv(fd int, iovs [][]byte, off int64) (int, error) {
	rn := 0
	for _, iov := range iovs {
		n, err := pread(fd, iov, off)
		if n > 0 {
			off += int64(n)
			rn += n
		}
		if err != nil {
			return rn, err
		}
		if n < len(iov) {
			break
		}
	}
	return rn, nil
}

func pwritev(fd int, iovs [][]byte, off int64) (int, error) {
	wn := 0
	for _, iov := range iovs {
		n, err := pwrite(fd, iov, off)
		if n > 0 {
			off += int64(n)
			wn += n
		}
		if err != nil {
			return wn, err
		}
		if n < len(iov) {
			break
		}
	}
	return wn, nil
}

func renameat(olddirfd int, oldpath string, newdirfd int, newpath string, flags int) error {
	oldpathptr, err := unix.BytePtrFromString(oldpath)
	if err != nil {
		return err
	}
	newpathptr, err := unix.BytePtrFromString(newpath)
	if err != nil {
		return err
	}
	return ignoreEINTR(func() error {
		// TODO: staticcheck complains that unix.Syscall6 is deprecated, but
		// there is no function for renameatx_np in the unix package, so we
		// have to fallback to using this function and disabling the check.
		_, _, err := unix.Syscall6(
			uintptr(unix.SYS_RENAMEATX_NP), //nolint:staticcheck
			uintptr(olddirfd),
			uintptr(unsafe.Pointer(oldpathptr)),
			uintptr(newdirfd),
			uintptr(unsafe.Pointer(newpathptr)),
			uintptr(flags),
			uintptr(0),
		)
		if err != 0 {
			return err
		}
		return nil
	})
}
