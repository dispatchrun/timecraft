package sandbox

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
)

var (
	ErrInterfaceNotFound = errors.New("network interface not found")
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
		return addrPortFromInet4(a)
	case *SockaddrInet6:
		return addrPortFromInet6(a)
	default:
		return netip.AddrPort{}
	}
}

func addrPortFromInet4(a *SockaddrInet4) netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom4(a.Addr), uint16(a.Port))
}

func addrPortFromInet6(a *SockaddrInet6) netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom16(a.Addr), uint16(a.Port))
}

func SockaddrFromAddrPort(addrPort netip.AddrPort) Sockaddr {
	addr := addrPort.Addr()
	port := addrPort.Port()
	if addr.Is4() {
		return &SockaddrInet4{
			Addr: addr.As4(),
			Port: int(port),
		}
	} else {
		return &SockaddrInet6{
			Addr: addr.As16(),
			Port: int(port),
		}
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
