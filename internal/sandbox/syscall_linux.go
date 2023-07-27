package sandbox

import "golang.org/x/sys/unix"

const (
	openPathFlags = unix.O_PATH | unix.O_DIRECTORY | unix.O_NOFOLLOW
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
