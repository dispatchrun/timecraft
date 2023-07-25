package sandbox

import (
	"syscall"

	"golang.org/x/sys/unix"
)

func pipe(fds *[2]int) error {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()

	if err := unix.Pipe(fds[:]); err != nil {
		return err
	}
	if err := unix.SetNonblock(fds[0], true); err != nil {
		closePipe(fds)
		return err
	}
	if err := unix.SetNonblock(fds[1], true); err != nil {
		closePipe(fds)
		return err
	}

	unix.CloseOnExec(fds[0])
	unix.CloseOnExec(fds[1])
	return nil
}
