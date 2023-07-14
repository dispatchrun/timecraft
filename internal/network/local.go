package network

import (
	"context"
	"net"
	"sync"
)

var (
	localAddrs = [2]net.Addr{
		&net.IPAddr{IP: net.IP{127, 0, 0, 1}},
		&net.IPAddr{IP: net.IP{15: 1}},
	}
)

type LocalOption func(*localNamespace)

func DialFunc(dial func(context.Context, string, string) (net.Conn, error)) LocalOption {
	return func(ns *localNamespace) { ns.dial = dial }
}

func ListenFunc(listen func(context.Context, string, string) (net.Listener, error)) LocalOption {
	return func(ns *localNamespace) { ns.listen = listen }
}

func ListenPacketFunc(listenPacket func(context.Context, string, string) (net.PacketConn, error)) LocalOption {
	return func(ns *localNamespace) { ns.listenPacket = listenPacket }
}

func NewLocalNamespace(host Namespace, opts ...LocalOption) Namespace {
	ns := &localNamespace{host: host}
	for _, opt := range opts {
		opt(ns)
	}
	return ns
}

type localNamespace struct {
	host         Namespace
	dial         func(context.Context, string, string) (net.Conn, error)
	listen       func(context.Context, string, string) (net.Listener, error)
	listenPacket func(context.Context, string, string) (net.PacketConn, error)
	lo0          localInterface
}

func (ns *localNamespace) InterfaceByIndex(index int) (Interface, error) {
	if index != 0 {
		return nil, errInterfaceIndexNotFound(index)
	}
	return &ns.lo0, nil
}

func (ns *localNamespace) InterfaceByName(name string) (Interface, error) {
	if name != "lo0" {
		return nil, errInterfaceNameNotFound(name)
	}
	return &ns.lo0, nil
}

func (ns *localNamespace) Interfaces() ([]Interface, error) {
	return []Interface{&ns.lo0}, nil
}

func (ns *localNamespace) bindInet4(sock *localSocket, addr *SockaddrInet4) error {
	switch {
	case isUnspecifiedInet4(addr), isLoopbackInet4(addr):
		return ns.lo0.bindInet4(sock, addr)
	default:
		return EADDRNOTAVAIL
	}
}

func (ns *localNamespace) bindInet6(sock *localSocket, addr *SockaddrInet6) error {
	switch {
	case isUnspecifiedInet6(addr), isLoopbackInet6(addr):
		return ns.lo0.bindInet6(sock, addr)
	default:
		return EADDRNOTAVAIL
	}
}

func (ns *localNamespace) lookupInet4(sock *localSocket, addr *SockaddrInet4) (*localSocket, error) {
	switch {
	case isUnspecifiedInet4(addr), isLoopbackInet4(addr):
		return ns.lo0.lookupInet4(sock, addr)
	default:
		return nil, ENETUNREACH
	}
}

func (ns *localNamespace) lookupInet6(sock *localSocket, addr *SockaddrInet6) (*localSocket, error) {
	switch {
	case isUnspecifiedInet6(addr), isLoopbackInet6(addr):
		return ns.lo0.lookupInet6(sock, addr)
	default:
		return nil, ENETUNREACH
	}
}

func (ns *localNamespace) unlinkInet4(sock *localSocket, addr *SockaddrInet4) {
	ns.lo0.unlinkInet4(sock, addr)
}

func (ns *localNamespace) unlinkInet6(sock *localSocket, addr *SockaddrInet6) {
	ns.lo0.unlinkInet6(sock, addr)
}

type localAddress struct {
	proto Protocol
	port  uint16
}

func localSockaddrInet4(proto Protocol, addr *SockaddrInet4) localAddress {
	return localAddress{
		proto: proto,
		port:  uint16(addr.Port),
	}
}

func localSockaddrInet6(proto Protocol, addr *SockaddrInet6) localAddress {
	return localAddress{
		proto: proto,
		port:  uint16(addr.Port),
	}
}

type localSocketTable struct {
	sockets map[localAddress]*localSocket
}

func (t *localSocketTable) bind(sock *localSocket, addr localAddress) (int, error) {
	if addr.port != 0 {
		if _, exist := t.sockets[addr]; exist {
			// TODO:
			// - SO_REUSEADDR
			// - SO_REUSEPORT
			return -1, EADDRNOTAVAIL
		}
	} else {
		var port int
		for port = 49152; port <= 65535; port++ {
			addr.port = uint16(port)
			if _, exist := t.sockets[addr]; !exist {
				break
			}
		}
		if port == 65535 {
			return -1, EADDRNOTAVAIL
		}
	}
	if t.sockets == nil {
		t.sockets = make(map[localAddress]*localSocket)
	}
	t.sockets[addr] = sock
	return int(addr.port), nil
}

func (t *localSocketTable) unlink(sock *localSocket, addr localAddress) {
	if t.sockets[addr] == sock {
		delete(t.sockets, addr)
	}
}

type localInterface struct {
	mutex sync.RWMutex
	ipv4  localSocketTable
	ipv6  localSocketTable
}

func (i *localInterface) Index() int {
	return 0
}

func (i *localInterface) MTU() int {
	return 1500
}

func (i *localInterface) Name() string {
	return "lo0"
}

func (i *localInterface) HardwareAddr() net.HardwareAddr {
	return net.HardwareAddr{}
}

func (i *localInterface) Flags() net.Flags {
	return net.FlagUp | net.FlagLoopback
}

func (i *localInterface) Addrs() ([]net.Addr, error) {
	return localAddrs[:], nil
}

func (i *localInterface) MulticastAddrs() ([]net.Addr, error) {
	return nil, nil
}

func (i *localInterface) bindInet4(sock *localSocket, addr *SockaddrInet4) error {
	link := localSockaddrInet4(sock.protocol, addr)
	name := &SockaddrInet4{Addr: addr.Addr}

	i.mutex.Lock()
	defer i.mutex.Unlock()

	port, err := i.ipv4.bind(sock, link)
	if err != nil {
		return err
	}

	name.Port = port
	sock.name.Store(name)
	sock.state.set(bound)
	return nil
}

func (i *localInterface) bindInet6(sock *localSocket, addr *SockaddrInet6) error {
	link := localSockaddrInet6(sock.protocol, addr)
	name := &SockaddrInet6{Addr: addr.Addr}

	i.mutex.Lock()
	defer i.mutex.Unlock()

	port, err := i.ipv6.bind(sock, link)
	if err != nil {
		return err
	}

	name.Port = port
	sock.name.Store(name)
	sock.state.set(bound)
	return nil
}

func (i *localInterface) lookupInet4(sock *localSocket, addr *SockaddrInet4) (*localSocket, error) {
	link := localSockaddrInet4(sock.protocol, addr)

	i.mutex.RLock()
	defer i.mutex.RUnlock()

	if peer := i.ipv4.sockets[link]; peer != nil {
		return peer, nil
	}
	return nil, EHOSTUNREACH
}

func (i *localInterface) lookupInet6(sock *localSocket, addr *SockaddrInet6) (*localSocket, error) {
	link := localSockaddrInet6(sock.protocol, addr)

	i.mutex.RLock()
	defer i.mutex.RUnlock()

	if peer := i.ipv6.sockets[link]; peer != nil {
		return peer, nil
	}
	return nil, EHOSTUNREACH
}

func (i *localInterface) unlinkInet4(sock *localSocket, addr *SockaddrInet4) {
	link := localSockaddrInet4(sock.protocol, addr)

	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.ipv4.unlink(sock, link)

	sock.name.Store(&sockaddrInet4Any)
	sock.state.unset(bound)
}

func (i *localInterface) unlinkInet6(sock *localSocket, addr *SockaddrInet6) {
	link := localSockaddrInet6(sock.protocol, addr)

	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.ipv6.unlink(sock, link)

	sock.name.Store(&sockaddrInet6Any)
	sock.state.unset(bound)
}
