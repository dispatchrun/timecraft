package network

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

func (s *hostSocket) Accept() (Socket, Sockaddr, error) {
	fd := s.fd.acquire()
	if fd < 0 {
		return nil, nil, EBADF
	}
	defer s.fd.release(fd)
	conn, addr, err := ignoreEINTR3(func() (int, unix.Sockaddr, error) {
		return unix.Accept4(fd, unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK)
	})
	if err != nil {
		return nil, nil, err
	}
	return newHostSocket(conn, s.family, s.socktype), addr, nil
}
