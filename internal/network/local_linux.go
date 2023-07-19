package network

import (
	"golang.org/x/sys/unix"
)

func socketpair(family, socktype, protocol int) ([2]int, error) {
	return ignoreEINTR2(func() ([2]int, error) {
		return unix.Socketpair(family, socktype|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, protocol)
	})
}
