package network

import (
	"fmt"
	"os"
	"runtime/debug"

	"golang.org/x/sys/unix"
)

func (s *socketFD) release(fd int) {
	s.releaseFunc(fd, closeTraceError)
}

func (s *socketFD) close() {
	s.closeFunc(closeTraceError)
}

func closeTraceError(fd int) {
	if err := unix.Close(fd); err != nil {
		fmt.Fprintf(os.Stderr, "close(%d) => %s\n", fd, err)
		debug.PrintStack()
	}
}
