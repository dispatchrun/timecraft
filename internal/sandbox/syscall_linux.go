package sandbox

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

const sizeOfDirent = 19

type dirent struct {
	ino    uint64
	off    uint64
	reclen uint16
	typ    uint8
}

func makeDirent(typ uint8, ino, off uint64, name string) dirent {
	return dirent{
		ino:    ino,
		off:    off,
		reclen: sizeOfDirent + uint16(len(name)) + 1,
		typ:    typ,
	}
}

const (
	O_DSYNC = unix.O_DSYNC
	O_RSYNC = unix.O_RSYNC
)

const (
	openPathFlags = unix.O_PATH | unix.O_DIRECTORY | unix.O_NOFOLLOW

	PATH_MAX   = 4096
	UTIME_NOW  = unix.UTIME_NOW
	UTIME_OMIT = unix.UTIME_OMIT
)

func accept(fd int) (int, Sockaddr, error) {
	return ignoreEINTR3(func() (int, Sockaddr, error) {
		return unix.Accept4(fd, unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK)
	})
}

func socket(family, socktype, protocol int) (int, error) {
	return ignoreEINTR2(func() (int, error) {
		return unix.Socket(family, socktype|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, protocol)
	})
}

func socketpair(family, socktype, protocol int) ([2]int, error) {
	return ignoreEINTR2(func() ([2]int, error) {
		return unix.Socketpair(family, socktype|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, protocol)
	})
}

func pipe(fds *[2]int) error {
	return unix.Pipe2(fds[:], unix.O_CLOEXEC|unix.O_NONBLOCK)
}

func fallocate(fd int, offset, length int64) error {
	return ignoreEINTR(func() error { return unix.Fallocate(fd, 0, offset, length) })
}

func fsync(fd int) error {
	return ignoreEINTR(func() error { return unix.Fsync(fd) })
}

func fdatasync(fd int) error {
	return ignoreEINTR(func() error { return unix.Fdatasync(fd) })
}

func futimens(fd int, ts *[2]unix.Timespec) error {
	// https://github.com/bminor/glibc/blob/master/sysdeps/unix/sysv/linux/futimens.c
	_, _, err := unix.Syscall6(
		uintptr(unix.SYS_UTIMENSAT),
		uintptr(fd),
		uintptr(0), // path=NULL
		uintptr(unsafe.Pointer(ts)),
		uintptr(0),
		uintptr(0),
		uintptr(0),
	)
	if err != 0 {
		return err
	}
	return nil
}

func freadlink(fd int, buf []byte) (int, error) {
	return readlinkat(fd, "", buf)
}

func lseek(fd int, offset int64, whence int) (int64, error) {
	return ignoreEINTR2(func() (int64, error) { return unix.Seek(fd, offset, whence) })
}

func readv(fd int, iovs [][]byte) (int, error) {
	return handleEINTR(func() (int, error) { return unix.Readv(fd, iovs) })
}

func writev(fd int, iovs [][]byte) (int, error) {
	return handleEINTR(func() (int, error) { return unix.Writev(fd, iovs) })
}

func preadv(fd int, iovs [][]byte, offset int64) (int, error) {
	return handleEINTR(func() (int, error) { return unix.Preadv(fd, iovs, offset) })
}

func pwritev(fd int, iovs [][]byte, offset int64) (int, error) {
	return handleEINTR(func() (int, error) { return unix.Pwritev(fd, iovs, offset) })
}
