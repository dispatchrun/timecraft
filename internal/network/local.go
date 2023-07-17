package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"

	"github.com/stealthrocket/timecraft/internal/ipam"
)

var ErrIPAM = errors.New("IP pool exhausted")

var localAddrs = [2]net.IPNet{
	net.IPNet{
		IP: net.IP{
			127, 0, 0, 1,
		},
		Mask: net.IPMask{
			255, 0, 0, 0,
		},
	},
	net.IPNet{
		IP: net.IP{
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
			0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
		},
		Mask: net.IPMask{
			0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
			0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		},
	},
}

type LocalOption func(*LocalNamespace)

func DialFunc(dial func(context.Context, string, string) (net.Conn, error)) LocalOption {
	return func(ns *LocalNamespace) { ns.dial = dial }
}

func ListenFunc(listen func(context.Context, string, string) (net.Listener, error)) LocalOption {
	return func(ns *LocalNamespace) { ns.listen = listen }
}

func ListenPacketFunc(listenPacket func(context.Context, string, string) (net.PacketConn, error)) LocalOption {
	return func(ns *LocalNamespace) { ns.listenPacket = listenPacket }
}

type LocalNetwork struct {
	addrs []net.IPNet
	ipams []ipam.Pool

	mutex  sync.RWMutex
	routes map[netip.Addr]*localInterface
}

func NewLocalNetwork(addrs ...*net.IPNet) *LocalNetwork {
	n := &LocalNetwork{
		addrs:  make([]net.IPNet, len(addrs)),
		ipams:  make([]ipam.Pool, len(addrs)),
		routes: make(map[netip.Addr]*localInterface),
	}
	for i, addr := range addrs {
		n.addrs[i] = *addr
		n.ipams[i] = ipam.NewPool(&n.addrs[i])
	}
	return n
}

func (n *LocalNetwork) CreateNamespace(host Namespace, opts ...LocalOption) (*LocalNamespace, error) {
	ns := &LocalNamespace{
		host: host,
		lo0: localInterface{
			index: 0,
			name:  "lo0",
			flags: net.FlagUp | net.FlagLoopback,
			addrs: localAddrs[:],
		},
		en0: localInterface{
			index: 1,
			name:  "en0",
			flags: net.FlagUp,
			addrs: make([]net.IPNet, 0, len(n.addrs)),
		},
	}

	for _, opt := range opts {
		opt(ns)
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	for i, ipam := range n.ipams {
		ip := ipam.GetIP()
		if ip == nil {
			n.detach(&ns.en0)
			return nil, fmt.Errorf("%s: %w", ipam, ErrIPAM)
		}
		ns.en0.addrs = append(ns.en0.addrs, net.IPNet{
			IP:   ip,
			Mask: n.addrs[i].Mask,
		})
	}

	ns.lo0.ports = make([]map[localPort]*localSocket, len(ns.lo0.addrs))
	ns.en0.ports = make([]map[localPort]*localSocket, len(ns.en0.addrs))
	n.attach(&ns.en0)
	ns.network.Store(n)
	return ns, nil
}

func (n *LocalNetwork) attach(iface *localInterface) {
	for _, ipnet := range iface.addrs {
		n.routes[addrFromIP(ipnet.IP)] = iface
	}
}

func (n *LocalNetwork) detach(iface *localInterface) {
	for i, ipnet := range iface.addrs {
		delete(n.routes, addrFromIP(ipnet.IP))
		n.ipams[i].PutIP(ipnet.IP)
	}
}

func (n *LocalNetwork) lookup(ip net.IP) *localInterface {
	return n.routes[addrFromIP(ip)]
}

func addrFromIP(ip net.IP) netip.Addr {
	if ipv4 := ip.To4(); ipv4 != nil {
		return netip.AddrFrom4(([4]byte)(ipv4))
	} else {
		return netip.AddrFrom16(([16]byte)(ip))
	}
}

type LocalNamespace struct {
	network atomic.Pointer[LocalNetwork]
	host    Namespace

	dial         func(context.Context, string, string) (net.Conn, error)
	listen       func(context.Context, string, string) (net.Listener, error)
	listenPacket func(context.Context, string, string) (net.PacketConn, error)

	lo0 localInterface
	en0 localInterface
}

func (ns *LocalNamespace) Socket(family Family, socktype Socktype, protocol Protocol) (Socket, error) {
	switch family {
	case INET, INET6:
	default:
		return ns.host.Socket(family, socktype, protocol)
	}
	s, err := ns.socket(family, socktype, protocol)
	if err != nil {
		return nil, err
	}
	return s, nil
}

func (ns *LocalNamespace) Detach() {
	if n := ns.network.Swap(nil); n != nil {
		n.mutex.Lock()
		n.detach(&ns.en0)
		n.mutex.Unlock()
	}
}

func (ns *LocalNamespace) InterfaceByIndex(index int) (Interface, error) {
	switch index {
	case ns.lo0.index:
		return &ns.lo0, nil
	case ns.en0.index:
		return &ns.en0, nil
	default:
		return nil, errInterfaceIndexNotFound(index)
	}
}

func (ns *LocalNamespace) InterfaceByName(name string) (Interface, error) {
	switch name {
	case ns.lo0.name:
		return &ns.lo0, nil
	case ns.en0.name:
		return &ns.en0, nil
	default:
		return nil, errInterfaceNameNotFound(name)
	}
}

func (ns *LocalNamespace) Interfaces() ([]Interface, error) {
	return []Interface{&ns.lo0, &ns.en0}, nil
}

func (ns *LocalNamespace) interfaces() []*localInterface {
	return []*localInterface{&ns.lo0, &ns.en0}
}

func (ns *LocalNamespace) bindInet4(sock *localSocket, addr *SockaddrInet4) error {
	return ns.bind(sock, addr.Addr[:], addr.Port)
}

func (ns *LocalNamespace) bindInet6(sock *localSocket, addr *SockaddrInet6) error {
	return ns.bind(sock, addr.Addr[:], addr.Port)
}

func (ns *LocalNamespace) bind(sock *localSocket, addr net.IP, port int) error {
	for _, iface := range ns.interfaces() {
		if err := iface.bind(sock, addr, port); err != nil {
			return err
		}
	}
	return nil
}

func (ns *LocalNamespace) lookupInet4(sock *localSocket, addr *SockaddrInet4) (*localSocket, error) {
	return ns.lookup(sock, addr.Addr[:], addr.Port)
}

func (ns *LocalNamespace) lookupInet6(sock *localSocket, addr *SockaddrInet6) (*localSocket, error) {
	return ns.lookup(sock, addr.Addr[:], addr.Port)
}

func (ns *LocalNamespace) lookup(sock *localSocket, addr net.IP, port int) (*localSocket, error) {
	for _, iface := range ns.interfaces() {
		if peer := iface.lookup(sock, addr, port); peer != nil {
			return peer, nil
		}
	}

	if addr.IsUnspecified() {
		return nil, EHOSTUNREACH
	}

	n := ns.network.Load()
	if n == nil {
		return nil, ENETUNREACH
	}

	n.mutex.RLock()
	iface := n.lookup(addr)
	n.mutex.RUnlock()

	if iface != nil {
		if peer := iface.lookup(sock, addr, port); peer != nil {
			return peer, nil
		}
		return nil, EHOSTUNREACH
	}
	return nil, ENETUNREACH
}

func (ns *LocalNamespace) unlinkInet4(sock *localSocket, addr *SockaddrInet4) {
	ns.unlink(sock, addr.Addr[:], addr.Port)
}

func (ns *LocalNamespace) unlinkInet6(sock *localSocket, addr *SockaddrInet6) {
	ns.unlink(sock, addr.Addr[:], addr.Port)
}

func (ns *LocalNamespace) unlink(sock *localSocket, addr net.IP, port int) {
	for _, iface := range ns.interfaces() {
		iface.unlink(sock, addr, port)
	}
}

type localPort struct {
	proto Protocol
	port  uint16
}

type localInterface struct {
	index int
	name  string
	haddr net.HardwareAddr
	flags net.Flags
	addrs []net.IPNet

	mutex sync.RWMutex
	ports []map[localPort]*localSocket
}

func (i *localInterface) Index() int {
	return i.index
}

func (i *localInterface) MTU() int {
	return 1500
}

func (i *localInterface) Name() string {
	return i.name
}

func (i *localInterface) HardwareAddr() net.HardwareAddr {
	return i.haddr
}

func (i *localInterface) Flags() net.Flags {
	return i.flags
}

func (i *localInterface) Addrs() ([]net.Addr, error) {
	addrs := make([]net.Addr, len(i.addrs))
	for j := range addrs {
		addrs[j] = &i.addrs[j]
	}
	return addrs, nil
}

func (i *localInterface) MulticastAddrs() ([]net.Addr, error) {
	return nil, nil
}

func (i *localInterface) bind(sock *localSocket, addr net.IP, port int) error {
	link := localPort{sock.protocol, uint16(port)}
	ipv4 := addr.To4()
	name := (Sockaddr)(nil)

	if ipv4 != nil {
		name = &SockaddrInet4{Addr: ([4]byte)(ipv4), Port: port}
	} else {
		name = &SockaddrInet6{Addr: ([16]byte)(addr), Port: port}
	}

	i.mutex.Lock()
	defer i.mutex.Unlock()

	if link.port != 0 {
		for j, a := range i.addrs {
			if !socketAndInterfaceMatch(addr, a.IP) {
				continue
			}
			if _, used := i.ports[j][link]; used {
				// TODO:
				// - SO_REUSEADDR
				// - SO_REUSEPORT
				return EADDRNOTAVAIL
			}
		}
	} else {
		var port int
	searchFreePort:
		for port = 49152; port <= 65535; port++ {
			link.port = uint16(port)
			for j, a := range i.addrs {
				if !socketAndInterfaceMatch(addr, a.IP) {
					continue
				}
				if _, used := i.ports[j][link]; used {
					continue searchFreePort
				}
			}
			break
		}
		if port == 65536 {
			return EADDRNOTAVAIL
		}
		switch a := name.(type) {
		case *SockaddrInet4:
			a.Port = port
		case *SockaddrInet6:
			a.Port = port
		}
	}

	for j := range i.ports {
		a := i.addrs[j]
		p := i.ports[j]

		if socketAndInterfaceMatch(addr, a.IP) {
			if p == nil {
				p = make(map[localPort]*localSocket)
				i.ports[j] = p
			}
			p[link] = sock
		}
	}

	sock.name.Store(name)
	sock.state.set(bound)
	return nil
}

func (i *localInterface) lookup(sock *localSocket, addr net.IP, port int) *localSocket {
	i.mutex.RLock()
	defer i.mutex.RUnlock()

	for j := range i.addrs {
		a := i.addrs[j]
		p := i.ports[j]

		if socketAndInterfaceMatch(addr, a.IP) {
			link := localPort{sock.protocol, uint16(port)}
			peer := p[link]
			if peer != nil {
				return peer
			}
		}
	}

	return nil
}

func (i *localInterface) unlink(sock *localSocket, addr net.IP, port int) {
	i.mutex.Lock()
	defer i.mutex.Unlock()

	for j := range i.addrs {
		a := i.addrs[j]
		p := i.ports[j]

		if socketAndInterfaceMatch(addr, a.IP) {
			link := localPort{sock.protocol, uint16(port)}
			peer := p[link]
			if sock == peer {
				delete(p, link)
			}
		}
	}
}

func socketAndInterfaceMatch(sock, iface net.IP) bool {
	return (sock.IsUnspecified() && family(sock) == family(iface)) || sock.Equal(iface)
}

func family(ip net.IP) Family {
	if ip.To4() != nil {
		return INET
	} else {
		return INET6
	}
}
