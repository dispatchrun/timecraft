package sandbox

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	O_DSYNC = unix.O_DSYNC
	O_RSYNC = unix.O_RSYNC
)

const (
	openPathFlags = unix.O_PATH | unix.O_DIRECTORY | unix.O_NOFOLLOW

	_PATH_MAX   = 4096
	_UTIME_NOW  = unix.UTIME_NOW
	_UTIME_OMIT = unix.UTIME_OMIT
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

func freadlink(fd int) (string, error) {
	return readlinkat(fd, "")
}
