package sandbox

import "golang.org/x/sys/unix"

func pipe(fds *[2]int) error {
	return unix.Pipe2(fds[:], unix.O_CLOEXEC|unix.O_NONBLOCK)
}
