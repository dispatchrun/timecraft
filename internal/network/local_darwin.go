package network

import (
	"syscall"

	"golang.org/x/sys/unix"
)

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

func closePair(fds *[2]int) {
	unix.Close(fds[0])
	unix.Close(fds[1])
	fds[0] = -1
	fds[1] = -1
}

func (s *localSocket) GetOptInt(level, name int) (int, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return 0, EBADF
	}
	defer s.fd0.release(fd)
	return getsockoptInt(fd, level, name)
}

func (s *localSocket) SetOptInt(level, name, value int) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)
	return setsockoptInt(fd, level, name, value)
}
