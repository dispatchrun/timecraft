package network

import "golang.org/x/sys/unix"

const (
	EADDRNOTAVAIL = unix.EADDRNOTAVAIL
	EAFNOSUPPORT  = unix.EAFNOSUPPORT
	EBADF         = unix.EBADF
	ECONNREFUSED  = unix.ECONNREFUSED
	ECONNRESET    = unix.ECONNRESET
	EHOSTUNREACH  = unix.EHOSTUNREACH
	EINVAL        = unix.EINVAL
	EINTR         = unix.EINTR
	EINPROGRESS   = unix.EINPROGRESS
	ENETUNREACH   = unix.ENETUNREACH
	ENOTCONN      = unix.ENOTCONN
)

const (
	UNIX  Family = unix.AF_UNIX
	INET  Family = unix.AF_INET
	INET6 Family = unix.AF_INET6
)

const (
	STREAM Socktype = unix.SOCK_STREAM
	DGRAM  Socktype = unix.SOCK_DGRAM
)

const (
	TRUNC   = unix.MSG_TRUNC
	PEEK    = unix.MSG_PEEK
	WAITALL = unix.MSG_WAITALL
)

const (
	SHUTRD = unix.SHUT_RD
	SHUTWR = unix.SHUT_WR
)

type Sockaddr = unix.Sockaddr
type SockaddrInet4 = unix.SockaddrInet4
type SockaddrInet6 = unix.SockaddrInet6
