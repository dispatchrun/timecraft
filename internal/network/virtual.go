package network

import (
	"errors"
	"net"
	"sync"
	"sync/atomic"

	"github.com/stealthrocket/timecraft/internal/ipam"
)

var (
	errIPv4Exhausted = errors.New("ipv4 pool exhausted")
	errIPv6Exhausted = errors.New("ipv6 pool exhausted")
)

type VirtualNetwork struct {
	mutex  sync.RWMutex
	host   Namespace
	ipnet4 *net.IPNet
	ipnet6 *net.IPNet
	ipam4  ipam.IPv4Pool
	ipam6  ipam.IPv6Pool
	iface4 map[ipam.IPv4]*virtualInterface
	iface6 map[ipam.IPv6]*virtualInterface
}

func NewVirtualNetwork(host Namespace, ipnet4, ipnet6 *net.IPNet) *VirtualNetwork {
	n := &VirtualNetwork{
		host:   host,
		ipnet4: ipnet4,
		ipnet6: ipnet6,
		iface4: make(map[ipam.IPv4]*virtualInterface),
		iface6: make(map[ipam.IPv6]*virtualInterface),
	}
	if ip := ipnet4.IP.To4(); ip != nil {
		ones, _ := ipnet4.Mask.Size()
		n.ipam4.Reset((ipam.IPv4)(ip), ones)
	}
	if ip := ipnet6.IP.To16(); ip != nil {
		ones, _ := ipnet6.Mask.Size()
		n.ipam6.Reset((ipam.IPv6)(ip), ones)
	}
	n.ipam4.Get()
	n.ipam6.Get()
	return n
}

func (n *VirtualNetwork) CreateNamespace() (*VirtualNamespace, error) {
	loIPv4 := ipam.IPv4{127, 0, 0, 1}
	loIPv6 := ipam.IPv6{15: 1}

	ns := &VirtualNamespace{
		lo0: virtualInterface{
			index: 0,
			name:  "lo0",
			ipv4:  loIPv4,
			ipv6:  loIPv6,
			flags: net.FlagUp | net.FlagLoopback,
		},
		en0: virtualInterface{
			index: 1,
			name:  "en0",
			flags: net.FlagUp,
		},
	}

	ns.net.Store(n)

	n.mutex.Lock()
	defer n.mutex.Unlock()

	var ok bool
	ns.en0.ipv4, ok = n.ipam4.Get()
	if !ok {
		return nil, errIPv4Exhausted
	}
	ns.en0.ipv6, ok = n.ipam6.Get()
	if !ok {
		n.ipam4.Put(ns.en0.ipv4)
		return nil, errIPv6Exhausted
	}

	n.iface4[ns.en0.ipv4] = &ns.en0
	n.iface6[ns.en0.ipv6] = &ns.en0
	return ns, nil
}

func (n *VirtualNetwork) Contains(ip net.IP) bool {
	return n.ipnet4.Contains(ip) || n.ipnet6.Contains(ip)
}

func (n *VirtualNetwork) containsIPv4(addr ipam.IPv4) bool {
	return n.ipnet4.Contains(addr[:])
}

func (n *VirtualNetwork) containsIPv6(addr ipam.IPv6) bool {
	return n.ipnet6.Contains(addr[:])
}

func (n *VirtualNetwork) lookupIPv4Interface(addr ipam.IPv4) *virtualInterface {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.iface4[addr]
}

func (n *VirtualNetwork) lookupIPv6Interface(addr ipam.IPv6) *virtualInterface {
	n.mutex.RLock()
	defer n.mutex.RUnlock()
	return n.iface6[addr]
}

func (n *VirtualNetwork) detachInterface(vi *virtualInterface) {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.iface4[vi.ipv4] == vi {
		delete(n.iface4, vi.ipv4)
		n.ipam4.Put(vi.ipv4)
	}
	if n.iface6[vi.ipv6] == vi {
		delete(n.iface6, vi.ipv6)
		n.ipam6.Put(vi.ipv6)
	}
}

type VirtualNamespace struct {
	net atomic.Pointer[VirtualNetwork]
	lo0 virtualInterface
	en0 virtualInterface
}

func (ns *VirtualNamespace) Detach() {
	if n := ns.net.Swap(nil); n != nil {
		n.detachInterface(&ns.en0)
	}
}

func (ns *VirtualNamespace) InterfaceByIndex(index int) (Interface, error) {
	switch index {
	case ns.lo0.index:
		return &ns.lo0, nil
	case ns.en0.index:
		return &ns.en0, nil
	default:
		return nil, errInterfaceIndexNotFound(index)
	}
}

func (ns *VirtualNamespace) InterfaceByName(name string) (Interface, error) {
	switch name {
	case ns.lo0.name:
		return &ns.lo0, nil
	case ns.en0.name:
		return &ns.en0, nil
	default:
		return nil, errInterfaceNameNotFound(name)
	}
}

func (ns *VirtualNamespace) Interfaces() ([]Interface, error) {
	return []Interface{&ns.lo0, &ns.en0}, nil
}

func (ns *VirtualNamespace) Socket(family Family, socktype Socktype, protocol Protocol) (Socket, error) {
	n := ns.net.Load()
	if n == nil {
		return nil, EAFNOSUPPORT
	}
	socket, err := n.host.Socket(family, socktype, protocol)
	if err != nil {
		return nil, err
	}
	switch family {
	case INET, INET6:
		socket = &virtualSocket{
			ns:    ns,
			base:  socket,
			proto: protocol,
		}
	}
	return socket, nil
}

func (ns *VirtualNamespace) bindInet4(socket *virtualSocket, host, addr *SockaddrInet4) error {
	switch addr.Addr {
	case [4]byte{}: // unspecified
		if err := ns.lo0.bindInet4(socket, host, addr); err != nil {
			return err
		}
		if err := ns.en0.bindInet4(socket, host, addr); err != nil {
			return err
		}
		return nil
	case ns.lo0.ipv4:
		return ns.lo0.bindInet4(socket, host, addr)
	case ns.en0.ipv4:
		return ns.en0.bindInet4(socket, host, addr)
	default:
		return EADDRNOTAVAIL
	}
}

func (ns *VirtualNamespace) bindInet6(socket *virtualSocket, host, addr *SockaddrInet6) error {
	switch addr.Addr {
	case [16]byte{}: // unspecified
		if err := ns.lo0.bindInet6(socket, host, addr); err != nil {
			return err
		}
		if err := ns.en0.bindInet6(socket, host, addr); err != nil {
			return err
		}
		return nil
	case ns.lo0.ipv6:
		return ns.lo0.bindInet6(socket, host, addr)
	case ns.en0.ipv6:
		return ns.en0.bindInet6(socket, host, addr)
	default:
		return EADDRNOTAVAIL
	}
}

func (ns *VirtualNamespace) lookupByHostInet4(socket *virtualSocket, host *SockaddrInet4) *virtualSocket {
	if peer := ns.lo0.lookupByHostInet4(socket, host); peer != nil {
		return peer
	}
	if peer := ns.en0.lookupByHostInet4(socket, host); peer != nil {
		return peer
	}
	if n := ns.net.Load(); n != nil {
		n.mutex.RLock()
		defer n.mutex.RUnlock()

		for _, i := range n.iface4 {
			if i == &ns.en0 {
				continue
			}
			if peer := i.lookupByHostInet4(socket, host); peer != nil {
				return peer
			}
		}
	}
	return nil
}

func (ns *VirtualNamespace) lookupByHostInet6(socket *virtualSocket, host *SockaddrInet6) *virtualSocket {
	if peer := ns.lo0.lookupByHostInet6(socket, host); peer != nil {
		return peer
	}
	if peer := ns.en0.lookupByHostInet6(socket, host); peer != nil {
		return peer
	}
	if n := ns.net.Load(); n != nil {
		n.mutex.RLock()
		defer n.mutex.RUnlock()

		for _, i := range n.iface6 {
			if i == &ns.en0 {
				continue
			}
			if peer := i.lookupByHostInet6(socket, host); peer != nil {
				return peer
			}
		}
	}
	return nil
}

func (ns *VirtualNamespace) lookupByAddrInet4(socket *virtualSocket, addr *SockaddrInet4) (*virtualSocket, error) {
	if isUnspecifiedInet4(addr) {
		return ns.lookupByPortInet4(socket, addr)
	}
	var peer *virtualSocket
	switch addr.Addr {
	case ns.lo0.ipv4:
		peer = ns.lo0.lookupByAddrInet4(socket, addr)
	case ns.en0.ipv4:
		peer = ns.en0.lookupByAddrInet4(socket, addr)
	default:
		n := ns.net.Load()
		if n == nil {
			return nil, ENETUNREACH
		}
		if !n.containsIPv4(addr.Addr) {
			return nil, nil
		}
		iface := n.lookupIPv4Interface(addr.Addr)
		if iface == nil {
			return nil, EHOSTUNREACH
		}
		peer = iface.lookupByAddrInet4(socket, addr)
	}
	if peer != nil {
		return peer, nil
	}
	return nil, EHOSTUNREACH
}

func (ns *VirtualNamespace) lookupByAddrInet6(socket *virtualSocket, addr *SockaddrInet6) (*virtualSocket, error) {
	if isUnspecifiedInet6(addr) {
		return ns.lookupByPortInet6(socket, addr)
	}
	var peer *virtualSocket
	switch addr.Addr {
	case ns.lo0.ipv6:
		peer = ns.lo0.lookupByAddrInet6(socket, addr)
	case ns.en0.ipv6:
		peer = ns.en0.lookupByAddrInet6(socket, addr)
	default:
		n := ns.net.Load()
		if n == nil {
			return nil, ENETUNREACH
		}
		if !n.containsIPv6(addr.Addr) {
			return nil, nil
		}
		iface := n.lookupIPv6Interface(addr.Addr)
		if iface == nil {
			return nil, EHOSTUNREACH
		}
		peer = iface.lookupByAddrInet6(socket, addr)
	}
	if peer != nil {
		return peer, nil
	}
	return nil, EHOSTUNREACH
}

func (ns *VirtualNamespace) lookupByPortInet4(socket *virtualSocket, addr *SockaddrInet4) (*virtualSocket, error) {
	if peer := ns.lo0.lookupByAddrInet4(socket, addr); peer != nil {
		return peer, nil
	}
	if peer := ns.en0.lookupByAddrInet4(socket, addr); peer != nil {
		return peer, nil
	}
	if n := ns.net.Load(); n != nil {
		n.mutex.RLock()
		defer n.mutex.RUnlock()

		for _, iface := range n.iface4 {
			if peer := iface.lookupByAddrInet4(socket, addr); peer != nil {
				return peer, nil
			}
		}
	}
	return nil, EHOSTUNREACH
}

func (ns *VirtualNamespace) lookupByPortInet6(socket *virtualSocket, addr *SockaddrInet6) (*virtualSocket, error) {
	if peer := ns.lo0.lookupByAddrInet6(socket, addr); peer != nil {
		return peer, nil
	}
	if peer := ns.en0.lookupByAddrInet6(socket, addr); peer != nil {
		return peer, nil
	}
	if n := ns.net.Load(); n != nil {
		n.mutex.RLock()
		defer n.mutex.RUnlock()

		for _, iface := range n.iface6 {
			if peer := iface.lookupByAddrInet6(socket, addr); peer != nil {
				return peer, nil
			}
		}
	}
	return nil, EHOSTUNREACH
}

func (ns *VirtualNamespace) unlinkInet4(socket *virtualSocket, host, addr *SockaddrInet4) {
	switch addr.Addr {
	case [4]byte{}:
		ns.lo0.unlinkInet4(socket, host, addr)
		ns.en0.unlinkInet4(socket, host, addr)
	case ns.lo0.ipv4:
		ns.lo0.unlinkInet4(socket, host, addr)
	case ns.en0.ipv4:
		ns.en0.unlinkInet4(socket, host, addr)
	}
}

func (ns *VirtualNamespace) unlinkInet6(socket *virtualSocket, host, addr *SockaddrInet6) {
	switch addr.Addr {
	case [16]byte{}:
		ns.lo0.unlinkInet6(socket, host, addr)
		ns.en0.unlinkInet6(socket, host, addr)
	case ns.lo0.ipv6:
		ns.lo0.unlinkInet6(socket, host, addr)
	case ns.en0.ipv6:
		ns.en0.unlinkInet6(socket, host, addr)
	}
}

type virtualAddress struct {
	proto Protocol
	port  uint16
}

func virtualSockaddrInet4(proto Protocol, addr *SockaddrInet4) virtualAddress {
	return virtualAddress{
		proto: proto,
		port:  uint16(addr.Port),
	}
}

func virtualSockaddrInet6(proto Protocol, addr *SockaddrInet6) virtualAddress {
	return virtualAddress{
		proto: proto,
		port:  uint16(addr.Port),
	}
}

type virtualAddressTable struct {
	host map[virtualAddress]*virtualSocket
	sock map[virtualAddress]*virtualSocket
}

func (t *virtualAddressTable) bind(socket *virtualSocket, ha, sa virtualAddress) (int, error) {
	if sa.port != 0 {
		if _, exist := t.sock[sa]; exist {
			// TODO:
			// - SO_REUSEADDR
			// - SO_REUSEPORT
			return -1, EADDRNOTAVAIL
		}
	} else {
		var port int
		for port = 49152; port <= 65535; port++ {
			sa.port = uint16(port)
			if _, exist := t.sock[sa]; !exist {
				break
			}
		}
		if port == 65535 {
			return -1, EADDRNOTAVAIL
		}
	}
	if t.host == nil {
		t.host = make(map[virtualAddress]*virtualSocket)
	}
	if t.sock == nil {
		t.sock = make(map[virtualAddress]*virtualSocket)
	}
	t.host[ha] = socket
	t.sock[sa] = socket
	return int(sa.port), nil
}

func (t *virtualAddressTable) unlink(socket *virtualSocket, ha, sa virtualAddress) {
	if t.host[ha] == socket {
		delete(t.host, ha)
	}
	if t.sock[sa] == socket {
		delete(t.sock, sa)
	}
}

type virtualInterface struct {
	index int
	name  string
	ipv4  ipam.IPv4
	ipv6  ipam.IPv6
	haddr net.HardwareAddr
	flags net.Flags

	mutex sync.RWMutex
	inet4 virtualAddressTable
	inet6 virtualAddressTable
}

func (i *virtualInterface) Index() int {
	return i.index
}

func (i *virtualInterface) MTU() int {
	return 1500
}

func (i *virtualInterface) Name() string {
	return i.name
}

func (i *virtualInterface) Addrs() ([]net.Addr, error) {
	ipv4 := &net.IPAddr{IP: net.IP(i.ipv4[:])}
	ipv6 := &net.IPAddr{IP: net.IP(i.ipv6[:])}
	return []net.Addr{ipv4, ipv6}, nil
}

func (i *virtualInterface) MulticastAddrs() ([]net.Addr, error) {
	return nil, nil
}

func (i *virtualInterface) HardwareAddr() net.HardwareAddr {
	return i.haddr
}

func (i *virtualInterface) Flags() net.Flags {
	return i.flags
}

func (i *virtualInterface) bindInet4(socket *virtualSocket, host, addr *SockaddrInet4) error {
	hostAddr := virtualSockaddrInet4(socket.proto, host)
	bindAddr := virtualSockaddrInet4(socket.proto, addr)
	name := &SockaddrInet4{Addr: addr.Addr}

	i.mutex.Lock()
	defer i.mutex.Unlock()

	port, err := i.inet4.bind(socket, hostAddr, bindAddr)
	if err != nil {
		return err
	}

	name.Port = port
	socket.host.Store(host)
	socket.name.Store(name)
	socket.bound = true
	return nil
}

func (i *virtualInterface) bindInet6(socket *virtualSocket, host, addr *SockaddrInet6) error {
	hostAddr := virtualSockaddrInet6(socket.proto, host)
	bindAddr := virtualSockaddrInet6(socket.proto, addr)
	name := &SockaddrInet6{Addr: addr.Addr}

	i.mutex.Lock()
	defer i.mutex.Unlock()

	port, err := i.inet6.bind(socket, hostAddr, bindAddr)
	if err != nil {
		return err
	}

	name.Port = port
	socket.host.Store(host)
	socket.name.Store(name)
	socket.bound = true
	return nil
}

func (i *virtualInterface) lookupByHostInet4(socket *virtualSocket, host *SockaddrInet4) *virtualSocket {
	va := virtualSockaddrInet4(socket.proto, host)
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.inet4.host[va]
}

func (i *virtualInterface) lookupByHostInet6(socket *virtualSocket, host *SockaddrInet6) *virtualSocket {
	va := virtualSockaddrInet6(socket.proto, host)
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.inet6.host[va]
}

func (i *virtualInterface) lookupByAddrInet4(socket *virtualSocket, addr *SockaddrInet4) *virtualSocket {
	va := virtualSockaddrInet4(socket.proto, addr)
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.inet4.sock[va]
}

func (i *virtualInterface) lookupByAddrInet6(socket *virtualSocket, addr *SockaddrInet6) *virtualSocket {
	va := virtualSockaddrInet6(socket.proto, addr)
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.inet6.sock[va]
}

func (i *virtualInterface) unlinkInet4(socket *virtualSocket, host, addr *SockaddrInet4) {
	hostAddr := virtualSockaddrInet4(socket.proto, host)
	sockAddr := virtualSockaddrInet4(socket.proto, addr)

	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.inet4.unlink(socket, hostAddr, sockAddr)
	socket.host.Store(&sockaddrInet4Any)
	socket.name.Store(&sockaddrInet4Any)
	socket.bound = false
}

func (i *virtualInterface) unlinkInet6(socket *virtualSocket, host, addr *SockaddrInet6) {
	hostAddr := virtualSockaddrInet6(socket.proto, host)
	sockAddr := virtualSockaddrInet6(socket.proto, addr)

	i.mutex.Lock()
	defer i.mutex.Unlock()

	i.inet6.unlink(socket, hostAddr, sockAddr)
	socket.host.Store(&sockaddrInet6Any)
	socket.name.Store(&sockaddrInet6Any)
	socket.bound = false
}

type virtualSocket struct {
	ns    *VirtualNamespace
	base  Socket
	host  atomic.Value
	name  atomic.Value
	peer  atomic.Value
	proto Protocol
	bound bool
}

func (s *virtualSocket) Family() Family {
	return s.base.Family()
}

func (s *virtualSocket) Type() Socktype {
	return s.base.Type()
}

func (s *virtualSocket) Fd() int {
	return s.base.Fd()
}

func (s *virtualSocket) Close() error {
	if s.bound {
		host := s.host.Load()
		name := s.name.Load()
		switch a := name.(type) {
		case *SockaddrInet4:
			s.ns.unlinkInet4(s, host.(*SockaddrInet4), a)
		case *SockaddrInet6:
			s.ns.unlinkInet6(s, host.(*SockaddrInet6), a)
		}
	}
	return s.base.Close()
}

func (s *virtualSocket) Bind(addr Sockaddr) error {
	if s.name.Load() != nil {
		return EINVAL
	}

	switch addr.(type) {
	case *SockaddrInet4:
		if s.Family() != INET {
			return EAFNOSUPPORT
		}
		_ = s.base.Bind(&sockaddrInet4Any)
	case *SockaddrInet6:
		if s.Family() != INET6 {
			return EAFNOSUPPORT
		}
		_ = s.base.Bind(&sockaddrInet6Any)
	default:
		return EINVAL
	}

	// The host socket was bound to a random port, we retrieve the address that
	// it got associated with.
	a, err := s.base.Name()
	if err != nil {
		return err
	}

	switch host := a.(type) {
	case *SockaddrInet4:
		return s.ns.bindInet4(s, host, addr.(*SockaddrInet4))
	case *SockaddrInet6:
		return s.ns.bindInet6(s, host, addr.(*SockaddrInet6))
	default:
		return EAFNOSUPPORT
	}
}

func (s *virtualSocket) bindAny() error {
	a, err := s.base.Name()
	if err != nil {
		return err
	}
	switch host := a.(type) {
	case *SockaddrInet4:
		return s.ns.bindInet4(s, host, &sockaddrInet4Any)
	case *SockaddrInet6:
		return s.ns.bindInet6(s, host, &sockaddrInet6Any)
	default:
		return EAFNOSUPPORT
	}
}

func (s *virtualSocket) Listen(backlog int) error {
	if err := s.base.Listen(backlog); err != nil {
		return err
	}
	if s.name.Load() == nil {
		return s.bindAny()
	}
	return nil
}

func (s *virtualSocket) Connect(addr Sockaddr) error {
	var peer *virtualSocket
	var err error
	switch a := addr.(type) {
	case *SockaddrInet4:
		peer, err = s.ns.lookupByAddrInet4(s, a)
	case *SockaddrInet6:
		peer, err = s.ns.lookupByAddrInet6(s, a)
	default:
		return EAFNOSUPPORT
	}
	if err != nil {
		return ECONNREFUSED
	}

	connectAddr := addr
	if peer != nil {
		switch a := peer.host.Load().(type) {
		case *SockaddrInet4:
			connectAddr = a
		case *SockaddrInet6:
			connectAddr = a
		default:
			return ECONNREFUSED
		}
	}

	err = s.base.Connect(connectAddr)
	if err != nil && err != EINPROGRESS {
		return err
	}

	s.peer.Store(addr)

	if s.name.Load() == nil {
		if err := s.bindAny(); err != nil {
			return err
		}
	}
	return err
}

func (s *virtualSocket) Accept() (Socket, Sockaddr, error) {
	base, addr, err := s.base.Accept()
	if err != nil {
		return nil, nil, err
	}
	conn := &virtualSocket{
		ns:   s.ns,
		base: base,
	}
	var peer *virtualSocket
	switch a := addr.(type) {
	case *SockaddrInet4:
		peer = s.ns.lookupByHostInet4(s, a)
	case *SockaddrInet6:
		peer = s.ns.lookupByHostInet6(s, a)
	}
	conn.host.Store(addr)
	if peer != nil {
		addr, _ = peer.name.Load().(Sockaddr)
	}
	conn.name.Store(s.name.Load())
	conn.peer.Store(addr)
	return conn, addr, nil
}

func (s *virtualSocket) Name() (Sockaddr, error) {
	switch name := s.name.Load().(type) {
	case *SockaddrInet4:
		return name, nil
	case *SockaddrInet6:
		return name, nil
	}
	switch s.Family() {
	case INET:
		return &sockaddrInet4Any, nil
	default:
		return &sockaddrInet6Any, nil
	}
}

func (s *virtualSocket) Peer() (Sockaddr, error) {
	switch peer := s.peer.Load().(type) {
	case *SockaddrInet4:
		return peer, nil
	case *SockaddrInet6:
		return peer, nil
	}
	return nil, ENOTCONN
}

func (s *virtualSocket) RecvFrom(iovs [][]byte, flags int) (int, int, Sockaddr, error) {
	n, flags, addr, err := s.base.RecvFrom(iovs, flags)
	if err != nil {
		return -1, 0, nil, err
	}
	if addr != nil {
		addr, err = s.toVirtualAddr(addr)
	}
	return n, flags, addr, err
}

func (s *virtualSocket) SendTo(iovs [][]byte, addr Sockaddr, flags int) (int, error) {
	if addr != nil {
		a, err := s.toHostAddr(addr)
		if err != nil {
			return -1, err
		}
		addr = a
	}
	n, err := s.base.SendTo(iovs, addr, flags)
	if s.name.Load() == nil {
		if err := s.bindAny(); err != nil {
			return n, err
		}
	}
	return n, err
}

func (s *virtualSocket) Shutdown(how int) error {
	return s.base.Shutdown(how)
}

func (s *virtualSocket) SetOptInt(level, name, value int) error {
	return s.base.SetOptInt(level, name, value)
}

func (s *virtualSocket) GetOptInt(level, name int) (int, error) {
	return s.base.GetOptInt(level, name)
}

func (s *virtualSocket) toHostAddr(addr Sockaddr) (Sockaddr, error) {
	var peer *virtualSocket
	var err error
	switch a := addr.(type) {
	case *SockaddrInet4:
		peer, err = s.ns.lookupByAddrInet4(s, a)
	case *SockaddrInet6:
		peer, err = s.ns.lookupByAddrInet6(s, a)
	default:
		return nil, EAFNOSUPPORT
	}
	if peer == nil {
		return addr, err
	}
	peerAddr, err := peer.Name()
	switch err {
	case nil:
		return peerAddr, nil
	case EBADF:
		return nil, ECONNRESET
	default:
		return nil, err
	}
}

func (s *virtualSocket) toVirtualAddr(addr Sockaddr) (Sockaddr, error) {
	var peer *virtualSocket
	switch a := addr.(type) {
	case *SockaddrInet4:
		peer = s.ns.lookupByHostInet4(s, a)
	case *SockaddrInet6:
		peer = s.ns.lookupByHostInet6(s, a)
	default:
		return nil, EAFNOSUPPORT
	}
	if peer != nil {
		return peer.name.Load().(Sockaddr), nil
	}
	// TODO: this races if the virtual peer was closed after sending a
	// datagram but before we looked it up on the interfaces.
	return addr, nil
}
