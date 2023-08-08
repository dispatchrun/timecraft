package container

import (
	"os"
	"syscall"
)

func duplicateFile(f *os.File, name string) (*os.File, error) {
	syscall.ForkLock.RLock()
	defer syscall.ForkLock.RUnlock()

	fd, err := syscall.Dup(int(f.Fd()))
	if err != nil {
		return nil, err
	}
	syscall.CloseOnExec(fd)
	return os.NewFile(uintptr(fd), name), nil
}
