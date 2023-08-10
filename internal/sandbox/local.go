package sandbox

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/stealthrocket/timecraft/internal/ipam"
)

var ErrIPAM = errors.New("IP pool exhausted")

var localAddrs = [2]netip.Prefix{
	netip.PrefixFrom(netip.AddrFrom4([4]byte{127, 0, 0, 1}), 8),
	netip.PrefixFrom(netip.IPv6Loopback(), 128),
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
	addrs []netip.Prefix
	ipams []ipam.Pool

	mutex  sync.RWMutex
	routes map[netip.Addr]*localInterface
}

func NewLocalNetwork(addrs ...netip.Prefix) *LocalNetwork {
	n := &LocalNetwork{
		addrs:  slices.Clone(addrs),
		ipams:  make([]ipam.Pool, len(addrs)),
		routes: make(map[netip.Addr]*localInterface),
	}
	for i, addr := range addrs {
		n.ipams[i] = ipam.NewPool(addr)
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
			addrs: make([]netip.Prefix, 0, len(n.addrs)),
		},
	}

	for _, opt := range opts {
		opt(ns)
	}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	for i, ipam := range n.ipams {
		ip, ok := ipam.GetAddr()
		if !ok {
			n.detach(&ns.en0)
			return nil, fmt.Errorf("%s: %w", ipam, ErrIPAM)
		}
		ns.en0.addrs = append(ns.en0.addrs, netip.PrefixFrom(ip, n.addrs[i].Bits()))
	}

	ns.lo0.ports = make([]map[localPort]*localSocket, len(ns.lo0.addrs))
	ns.en0.ports = make([]map[localPort]*localSocket, len(ns.en0.addrs))
	n.attach(&ns.en0)
	ns.network.Store(n)
	return ns, nil
}

func (n *LocalNetwork) attach(iface *localInterface) {
	for _, prefix := range iface.addrs {
		n.routes[prefix.Addr()] = iface
	}
}

func (n *LocalNetwork) detach(iface *localInterface) {
	for i, prefix := range iface.addrs {
		addr := prefix.Addr()
		delete(n.routes, addr)
		n.ipams[i].PutAddr(addr)
	}
}

func (n *LocalNetwork) lookup(addr netip.Addr) *localInterface {
	return n.routes[addr]
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
		if ns.host != nil {
			return ns.host.Socket(family, socktype, protocol)
		} else {
			return nil, EAFNOSUPPORT
		}
	}

	// TODO: remove
	//
	// We make this special case because datagram sockets are used for DNS
	// resolution, and resolvers read /etc/resolv.conf to determine the address
	// of the DNS server to contact. The address is usually localhost, which
	// breaks if there is no DNS server listening on localhost. Since the local
	// network creates a virtual loopback, we would need to run a DNS server on
	// this address to support name resolution. This also likely means that the
	// sandbox must support mounting a file at /etc/resolv.conf to expose the
	// server details to the resolver, otherwise the value exposed by the system
	// could differ from the timecraft virtual network configuration.
	switch socktype {
	case DGRAM:
		if ns.host != nil {
			return ns.host.Socket(family, socktype, protocol)
		}
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
	return ns.bind(sock, addrPortFromInet4(addr))
}

func (ns *LocalNamespace) bindInet6(sock *localSocket, addr *SockaddrInet6) error {
	return ns.bind(sock, addrPortFromInet6(addr))
}

func (ns *LocalNamespace) bind(sock *localSocket, addrPort netip.AddrPort) error {
	for _, iface := range ns.interfaces() {
		if err := iface.bind(sock, addrPort); err != nil {
			return err
		}
	}
	return nil
}

func (ns *LocalNamespace) lookupInet4(sock *localSocket, addr *SockaddrInet4) (*localSocket, error) {
	return ns.lookup(sock, addrPortFromInet4(addr))
}

func (ns *LocalNamespace) lookupInet6(sock *localSocket, addr *SockaddrInet6) (*localSocket, error) {
	return ns.lookup(sock, addrPortFromInet6(addr))
}

func (ns *LocalNamespace) lookup(sock *localSocket, addrPort netip.AddrPort) (*localSocket, error) {
	for _, iface := range ns.interfaces() {
		if peer := iface.lookup(sock, addrPort); peer != nil {
			return peer, nil
		}
	}

	addr := addrPort.Addr()
	if addr.IsUnspecified() {
		return nil, EHOSTUNREACH
	}

	for _, iface := range ns.interfaces() {
		if iface.contains(addr) {
			return nil, EHOSTUNREACH
		}
	}

	n := ns.network.Load()
	if n == nil {
		return nil, ENETUNREACH
	}

	n.mutex.RLock()
	iface := n.lookup(addr)
	n.mutex.RUnlock()

	if iface != nil {
		if peer := iface.lookup(sock, addrPort); peer != nil {
			return peer, nil
		}
		return nil, EHOSTUNREACH
	}
	return nil, ENETUNREACH
}

func (ns *LocalNamespace) unlinkInet4(sock *localSocket, addr *SockaddrInet4) {
	ns.unlink(sock, addrPortFromInet4(addr))
}

func (ns *LocalNamespace) unlinkInet6(sock *localSocket, addr *SockaddrInet6) {
	ns.unlink(sock, addrPortFromInet6(addr))
}

func (ns *LocalNamespace) unlink(sock *localSocket, addrPort netip.AddrPort) {
	for _, iface := range ns.interfaces() {
		iface.unlink(sock, addrPort)
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
	addrs []netip.Prefix

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
	for j, prefix := range i.addrs {
		addr := prefix.Addr()
		ones := prefix.Bits()
		bits := addr.BitLen()
		addrs[j] = &net.IPNet{
			IP:   addr.AsSlice(),
			Mask: net.CIDRMask(ones, bits),
		}
	}
	return addrs, nil
}

func (i *localInterface) MulticastAddrs() ([]net.Addr, error) {
	return nil, nil
}

func (i *localInterface) contains(addr netip.Addr) bool {
	for j := range i.addrs {
		if i.addrs[j].Addr() == addr {
			return true
		}
	}
	return false
}

func (i *localInterface) bind(sock *localSocket, addrPort netip.AddrPort) error {
	link := localPort{sock.protocol, addrPort.Port()}
	name := SockaddrFromAddrPort(addrPort)
	addr := addrPort.Addr()

	i.mutex.Lock()
	defer i.mutex.Unlock()

	if link.port != 0 {
		for j, a := range i.addrs {
			if !socketAndInterfaceMatch(addr, a.Addr()) {
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
				if !socketAndInterfaceMatch(addr, a.Addr()) {
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

		if socketAndInterfaceMatch(addr, a.Addr()) {
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

func (i *localInterface) lookup(sock *localSocket, addrPort netip.AddrPort) *localSocket {
	addr := addrPort.Addr()

	i.mutex.RLock()
	defer i.mutex.RUnlock()

	for j := range i.addrs {
		a := i.addrs[j]
		p := i.ports[j]

		if socketAndInterfaceMatch(addr, a.Addr()) {
			link := localPort{sock.protocol, addrPort.Port()}
			peer := p[link]
			if peer != nil {
				return peer
			}
		}
	}

	return nil
}

func (i *localInterface) unlink(sock *localSocket, addrPort netip.AddrPort) {
	addr := addrPort.Addr()

	i.mutex.Lock()
	defer i.mutex.Unlock()

	for j := range i.addrs {
		a := i.addrs[j]
		p := i.ports[j]

		if socketAndInterfaceMatch(addr, a.Addr()) {
			link := localPort{sock.protocol, addrPort.Port()}
			peer := p[link]
			if sock == peer {
				delete(p, link)
			}
		}
	}
}

func socketAndInterfaceMatch(sock, iface netip.Addr) bool {
	return (sock.IsUnspecified() && family(sock) == family(iface)) || sock == iface
}

func family(addr netip.Addr) Family {
	if addr.Is4() {
		return INET
	} else {
		return INET6
	}
}
