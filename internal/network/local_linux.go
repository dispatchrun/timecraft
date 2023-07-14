package network

import "golang.org/x/sys/unix"

func socketpair(family, socktype, protocol int) ([2]int, error) {
	return ignoreEINTR2(func() ([2]int, error) {
		return unix.Socketpair(family, socktype|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, protocol)
	})
}

func (s *localSocket) GetOptInt(level, name int) (int, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return 0, EBADF
	}
	defer s.fd0.release(fd)

	switch level {
	case unix.SOL_SOCKET:
		switch name {
		case unix.SO_DOMAIN:
			return int(s.family), nil
		case unix.SO_BINDTODEVICE:
			return 0, ENOPROTOOPT
		}
	}

	return getsockoptInt(fd, level, name)
}

func (s *localSocket) SetOptInt(level, name, value int) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)

	switch level {
	case unix.SOL_SOCKET:
		switch name {
		case unix.SO_BINDTODEVICE:
			return 0, ENOPROTOOPT
		}
	}

	return setsockoptInt(fd, level, name, value)
}
