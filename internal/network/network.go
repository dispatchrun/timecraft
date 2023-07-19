package network

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
)

var (
	ErrInterfaceNotFound = errors.New("network interface not found")
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

	RecvFrom(iovs [][]byte, flags int) (n, rflags int, addr Sockaddr, err error)

	SendTo(iovs [][]byte, addr Sockaddr, flags int) (int, error)

	Shutdown(how int) error

	GetOptInt(level, name int) (int, error)

	GetOptString(level, name int) (string, error)

	SetOptInt(level, name, value int) error

	SetOptString(level, name int, value string) error
}

type Socktype uint8

type Family uint8

func (f Family) String() string {
	switch f {
	case UNIX:
		return "UNIX"
	case INET:
		return "INET"
	case INET6:
		return "INET6"
	default:
		return "UNSPEC"
	}
}

type Protocol uint16

const (
	NOPROTO Protocol = 0
	TCP     Protocol = 6
	UDP     Protocol = 17
)

func (p Protocol) String() string {
	switch p {
	case NOPROTO:
		return "NOPROTO"
	case TCP:
		return "TCP"
	case UDP:
		return "UDP"
	default:
		return "UNKNOWN"
	}
}

func (p Protocol) Network() string {
	switch p {
	case TCP:
		return "tcp"
	case UDP:
		return "udp"
	default:
		return "ip"
	}
}

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

func SockaddrAddr(sa Sockaddr) netip.Addr {
	switch a := sa.(type) {
	case *SockaddrInet4:
		return netip.AddrFrom4(a.Addr)
	case *SockaddrInet6:
		return netip.AddrFrom16(a.Addr)
	default:
		return netip.Addr{}
	}
}

func SockaddrAddrPort(sa Sockaddr) netip.AddrPort {
	switch a := sa.(type) {
	case *SockaddrInet4:
		return netip.AddrPortFrom(netip.AddrFrom4(a.Addr), uint16(a.Port))
	case *SockaddrInet6:
		return netip.AddrPortFrom(netip.AddrFrom16(a.Addr), uint16(a.Port))
	default:
		return netip.AddrPort{}
	}
}

func errInterfaceIndexNotFound(index int) error {
	return fmt.Errorf("index=%d: %w", index, ErrInterfaceNotFound)
}

func errInterfaceNameNotFound(name string) error {
	return fmt.Errorf("name=%q: %w", name, ErrInterfaceNotFound)
}

var (
	sockaddrInet4Any SockaddrInet4
	sockaddrInet6Any SockaddrInet6
)
