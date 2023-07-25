package sandbox

import (
	"golang.org/x/sys/unix"
)

const (
	O_RDONLY = unix.O_RDONLY
	O_WRONLY = unix.O_WRONLY
	O_RDWR   = unix.O_RDWR
	O_APPEND = unix.O_APPEND
	O_CREAT  = unix.O_CREAT
	O_EXCL   = unix.O_EXCL
	O_TRUNC  = unix.O_TRUNC
)
