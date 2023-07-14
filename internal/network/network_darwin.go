package network

import "golang.org/x/sys/unix"

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
