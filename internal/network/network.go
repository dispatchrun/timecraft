package network

import (
	"net"
)

type Socket interface {
	Family() Family

	Type() Socktype

	Fd() int

	Close() error

	Bind(addr Sockaddr) error

	Listen(backlog int) error

	Connect(addr Sockaddr) error

	Accept() (Socket, Sockaddr, error)

	Name() (Sockaddr, error)

	Peer() (Sockaddr, error)

	RecvFrom(iovs [][]byte, oob []byte, flags int) (n, oobn, rflags int, addr Sockaddr, err error)

	SendTo(iovs [][]byte, oob []byte, addr Sockaddr, flags int) (int, error)

	Shutdown(how int) error

	SetOption(level, name, value int) error

	GetOption(level, name int) (int, error)
}

type Socktype uint8

type Family uint8

type Protocol uint16

const (
	UNSPEC Protocol = 0
	TCP    Protocol = 6
	UDP    Protocol = 17
)

type Namespace interface {
	InterfaceByIndex(index int) (Interface, error)

	InterfaceByName(name string) (Interface, error)

	Interfaces() ([]Interface, error)

	Socket(family Family, socktype Socktype, protocol Protocol) (Socket, error)
}

type Interface interface {
	Index() int

	MTU() int

	Name() string

	HardwareAddr() net.HardwareAddr

	Flags() net.Flags

	Addrs() ([]net.Addr, error)

	MulticastAddrs() ([]net.Addr, error)
}

func SockaddrFamily(sa Sockaddr) Family {
	switch sa.(type) {
	case *SockaddrInet4:
		return INET
	case *SockaddrInet6:
		return INET6
	default:
		return UNIX
	}
}

func isUnspecified(sa Sockaddr) bool {
	switch a := sa.(type) {
	case *SockaddrInet4:
		return isUnspecifiedInet4(a)
	case *SockaddrInet6:
		return isUnspecifiedInet6(a)
	default:
		return true
	}
}

func isUnspecifiedInet4(sa *SockaddrInet4) bool {
	return sa.Addr == [4]byte{}
}

func isUnspecifiedInet6(sa *SockaddrInet6) bool {
	return sa.Addr == [16]byte{}
}
