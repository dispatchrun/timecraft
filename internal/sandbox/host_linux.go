package sandbox

import "golang.org/x/sys/unix"

func (hostNamespace) Socket(family Family, socktype Socktype, protocol Protocol) (Socket, error) {
	fd, err := ignoreEINTR2(func() (int, error) {
		return unix.Socket(int(family), int(socktype)|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, int(protocol))
	})
	if err != nil {
		return nil, err
	}
	return newHostSocket(fd, family, socktype), nil
}

func accept(fd int) (int, Sockaddr, error) {
	return ignoreEINTR3(func() (int, Sockaddr, error) {
		return unix.Accept4(fd, unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK)
	})
}
