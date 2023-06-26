//nolint:unused
package sandbox

import (
	"context"
	"fmt"
	"math"
	"net"
	"net/netip"
	"sync"

	"github.com/stealthrocket/wasi-go"
)

// Dial opens a connection to a listening socket on the guest module network.
//
// This function has a signature that matches the one commonly used in the
// Go standard library as a hook to customize how and where network connections
// are estalibshed. The intent is for this function to be used when the host
// needs to establish a connection to the guest, maybe indirectly such as using
// a http.Transport and setting this method as the transport's dial function.
func (s *System) Dial(ctx context.Context, network, address string) (net.Conn, error) {
	var protocol protocol

	switch network {
	case "tcp", "tcp4", "tcp6":
		protocol = tcp
	case "udp", "udp4", "udp6":
		protocol = udp
	case "unix":
	default:
		return nil, newDialError(network, net.UnknownNetworkError(network))
	}

	switch protocol {
	case tcp, udp:
		addrPort, err := netip.ParseAddrPort(address)
		if err != nil {
			return nil, newDialError(network, &net.ParseError{
				Type: "connect address",
				Text: address,
			})
		}

		addr := addrPort.Addr()
		port := addrPort.Port()
		if port == 0 {
			return nil, newDialError(network, &net.AddrError{
				Err:  "missing port in connect address",
				Addr: address,
			})
		}

		if addr.Is4() {
			if network == "tcp6" || network == "udp6" {
				return nil, newDialError(network, net.InvalidAddrError(address))
			}
			return connect(s, &s.ipv4, netaddr[ipv4]{
				protocol: protocol,
				sockaddr: ipv4{
					addr: addr.As4(),
					port: uint32(port),
				},
			})
		} else {
			if network == "tcp4" || network == "udp4" {
				return nil, newDialError(network, net.InvalidAddrError(address))
			}
			return connect(s, &s.ipv6, netaddr[ipv6]{
				protocol: protocol,
				sockaddr: ipv6{
					addr: addr.As16(),
					port: uint32(port),
				},
			})
		}

	default:
		return connect(s, &s.unix, netaddr[unix]{
			sockaddr: unix{
				name: address,
			},
		})
	}
}

// Listen opens a listening socket on the network stack of the guest module,
// returning a net.Listener that the host can use to receive connections to the
// given network address.
//
// The returned listener does not exist in the guest module file table, which
// means that the guest cannot shut it down, allowing the host ot have full
// control over the lifecycle of the underlying socket.
func (s *System) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		addr, port, err := parseListenAddrPort(network, address)
		if err != nil {
			return nil, err
		}

		if addr.Is4() {
			return listen(s, &s.ipv4, netaddr[ipv4]{
				protocol: tcp,
				sockaddr: ipv4{
					addr: addr.As4(),
					port: uint32(port),
				},
			})
		} else {
			return listen(s, &s.ipv6, netaddr[ipv6]{
				protocol: tcp,
				sockaddr: ipv6{
					addr: addr.As16(),
					port: uint32(port),
				},
			})
		}

	case "unix":
		return listen(s, &s.unix, netaddr[unix]{
			sockaddr: unix{
				name: address,
			},
		})

	default:
		return nil, newListenError(network, net.UnknownNetworkError(network))
	}
}

// ListenPacket is like Listen but for datagram connections.
//
// The supported networks are "udp", "udp4", and "udp6".
func (s *System) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	switch network {
	case "udp", "udp4", "udp6":
		addr, port, err := parseListenAddrPort(network, address)
		if err != nil {
			return nil, err
		}

		if addr.Is4() {
			return listenPacket(s, &s.ipv4, netaddr[ipv4]{
				protocol: udp,
				sockaddr: ipv4{
					addr: addr.As4(),
					port: uint32(port),
				},
			})
		} else {
			return listenPacket(s, &s.ipv6, netaddr[ipv6]{
				protocol: udp,
				sockaddr: ipv6{
					addr: addr.As16(),
					port: uint32(port),
				},
			})
		}

	default:
		return nil, newListenError(network, net.UnknownNetworkError(network))
	}
}

func parseListenAddrPort(network, address string) (addr netip.Addr, port uint16, err error) {
	h, p, err := net.SplitHostPort(address)
	if err != nil {
		return addr, port, newListenError(network, &net.ParseError{
			Type: "listen address",
			Text: address,
		})
	}

	// Allow omitting the address to let the system select the best match.
	if h == "" {
		if network == "tcp6" || network == "udp6" {
			h = "[::]"
		} else {
			h = "0.0.0.0"
		}
	}

	addrPort, err := netip.ParseAddrPort(net.JoinHostPort(h, p))
	if err != nil {
		return addr, port, newListenError(network, &net.ParseError{
			Type: "listen address",
			Text: address,
		})
	}

	addr = addrPort.Addr()
	port = addrPort.Port()

	if addr.Is4() {
		if network == "tcp6" || network == "udp6" {
			err = newListenError(network, net.InvalidAddrError(address))
		}
	} else {
		if network == "tcp4" || network == "udp4" {
			err = newListenError(network, net.InvalidAddrError(address))
		}
	}

	return addr, port, err
}

func newDialError(network string, err error) error {
	return &net.OpError{Op: "dial", Net: network, Err: err}
}

func newListenError(network string, err error) error {
	return &net.OpError{Op: "listen", Net: network, Err: err}
}

func connect[N network[T], T sockaddr](s *System, n N, addr netaddr[T]) (net.Conn, error) {
	makeError := func(errno wasi.Errno) error {
		netAddr := addr.netAddr()
		return &net.OpError{
			Op:   "connect",
			Net:  netAddr.Network(),
			Addr: netAddr,
			Err:  errno.Syscall(),
		}
	}
	sock := n.socket(addr)
	if sock == nil {
		return nil, makeError(wasi.ECONNREFUSED)
	}
	var zero T
	conn := sock.newSocket()
	errno := sock.connect(nil, conn, zero, addr.sockaddr)
	if errno != wasi.ESUCCESS {
		return nil, makeError(errno)
	}
	if conn.typ == datagram {
		return newPacketConn(conn), nil
	} else {
		return newHostConn(conn), nil
	}
}

func listen[N network[T], T sockaddr](s *System, n N, addr netaddr[T]) (net.Listener, error) {
	accept := make(chan *socket[T], 128)
	socket := newSocket[T](n, stream, addr.protocol, s.lock, s.poll)
	socket.flags = socket.flags.with(sockListen)
	socket.accept = accept

	if errno := n.bind(addr.sockaddr, socket); errno != wasi.ESUCCESS {
		netAddr := addr.netAddr()
		return nil, &net.OpError{
			Op:   "listen",
			Net:  netAddr.Network(),
			Addr: netAddr,
			Err:  errno.Syscall(),
		}
	}

	lstn := &listener[T]{
		accept: accept,
		socket: socket,
	}
	return lstn, nil
}

func listenPacket[N network[T], T sockaddr](s *System, n N, addr netaddr[T]) (net.PacketConn, error) {
	socket := newSocket[T](n, datagram, addr.protocol, s.lock, s.poll)
	socket.allocateBuffersIfNil()

	if errno := n.bind(addr.sockaddr, socket); errno != wasi.ESUCCESS {
		netAddr := addr.netAddr()
		return nil, &net.OpError{
			Op:   "listen",
			Net:  netAddr.Network(),
			Addr: netAddr,
			Err:  errno.Syscall(),
		}
	}

	return newPacketConn(socket), nil
}

type listener[T sockaddr] struct {
	accept chan *socket[T]
	socket *socket[T]
}

func (l *listener[T]) Close() error {
	l.socket.close()
	return nil
}

func (l *listener[T]) Addr() net.Addr {
	return l.socket.laddr.netAddr(l.socket.proto)
}

func (l *listener[T]) Accept() (net.Conn, error) {
	socket, ok := <-l.accept
	if !ok {
		return nil, net.ErrClosed
	}
	if socket.flags.has(sockHost) {
		return newGuestConn(socket), nil
	} else {
		return newHostConn(socket), nil
	}
}

type sockaddr interface {
	fmt.Stringer
	family() wasi.ProtocolFamily
	sockAddr() wasi.SocketAddress
	netAddr(protocol) net.Addr
	comparable
}

func makeIPNetAddr(proto protocol, ip net.IP, port int) net.Addr {
	switch proto {
	case tcp:
		return &net.TCPAddr{IP: ip, Port: port}
	case udp:
		return &net.UDPAddr{IP: ip, Port: port}
	default:
		return nil
	}
}

type inaddr[T sockaddr] interface {
	addrPort() netip.AddrPort
	withAddr(netip.Addr) T
	withPort(int) T
	unspecified() T
	sockaddr
}

type ipv4 struct {
	addr [4]byte
	port uint32
}

func (inaddr ipv4) String() string {
	return inaddr.sockAddr().String()
}

func (inaddr ipv4) family() wasi.ProtocolFamily {
	return wasi.InetFamily
}

func (inaddr ipv4) addrPort() netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom4(inaddr.addr), uint16(inaddr.port))
}

func (inaddr ipv4) withAddr(addr netip.Addr) ipv4 {
	return ipv4{addr: addr.As4(), port: inaddr.port}
}

func (inaddr ipv4) withPort(port int) ipv4 {
	return ipv4{addr: inaddr.addr, port: uint32(port)}
}

func (inaddr ipv4) unspecified() ipv4 {
	return ipv4{port: inaddr.port}
}

func (inaddr ipv4) sockAddr() wasi.SocketAddress {
	return &wasi.Inet4Address{Addr: inaddr.addr, Port: int(inaddr.port)}
}

func (inaddr ipv4) netAddr(proto protocol) net.Addr {
	return makeIPNetAddr(proto, net.IP(inaddr.addr[:]), int(inaddr.port))
}

type ipv6 struct {
	addr [16]byte
	port uint32
}

func (inaddr ipv6) String() string {
	return inaddr.sockAddr().String()
}

func (inaddr ipv6) family() wasi.ProtocolFamily {
	return wasi.Inet6Family
}

func (inaddr ipv6) addrPort() netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom16(inaddr.addr), uint16(inaddr.port))
}

func (inaddr ipv6) withAddr(addr netip.Addr) ipv6 {
	return ipv6{addr: addr.As16(), port: inaddr.port}
}

func (inaddr ipv6) withPort(port int) ipv6 {
	return ipv6{addr: inaddr.addr, port: uint32(port)}
}

func (inaddr ipv6) unspecified() ipv6 {
	return ipv6{port: inaddr.port}
}

func (inaddr ipv6) sockAddr() wasi.SocketAddress {
	return &wasi.Inet6Address{Addr: inaddr.addr, Port: int(inaddr.port)}
}

func (inaddr ipv6) netAddr(proto protocol) net.Addr {
	return makeIPNetAddr(proto, net.IP(inaddr.addr[:]), int(inaddr.port))
}

type unix struct {
	name string
}

func (unaddr unix) String() string {
	return unaddr.name
}

func (unaddr unix) family() wasi.ProtocolFamily {
	return wasi.UnixFamily
}

func (unaddr unix) sockAddr() wasi.SocketAddress {
	return &wasi.UnixAddress{Name: unaddr.name}
}

func (unaddr unix) netAddr(protocol) net.Addr {
	return &net.UnixAddr{Net: "unix", Name: unaddr.name}
}

type protocol wasi.Protocol

const (
	ip  = protocol(wasi.IPProtocol)
	tcp = protocol(wasi.TCPProtocol)
	udp = protocol(wasi.UDPProtocol)
)

func (proto protocol) String() string {
	switch proto {
	case tcp:
		return "tcp"
	case udp:
		return "udp"
	default:
		return "unknown"
	}
}

type netaddr[T sockaddr] struct {
	protocol protocol
	sockaddr T
}

func (n netaddr[T]) netAddr() net.Addr {
	return n.sockaddr.netAddr(n.protocol)
}

// The network interface abstracts the underlying network that sockets are
// created on.
type network[T sockaddr] interface {
	// Returns the address of this network.
	address() T
	// Returns true if the network contains the given address.
	contains(T) bool
	// Returns true if the network supports the given protocol.
	supports(protocol) bool
	// Constructs a socket address for the network from a net.Addr.
	netaddr(addr net.Addr) (T, wasi.Errno)
	// Constructs a socket address for the network from a wasi.SocketAddress.
	sockaddr(addr wasi.SocketAddress) (T, wasi.Errno)
	// Returns the socket associated with the given network address.
	socket(addr netaddr[T]) *socket[T]
	// Binds a socket to an address. Unlink must be called to remove the
	// socket when it's closed (this is done automatically by the socket's
	// close method).
	//
	// Bind sets sock.bound to the address that the socket was bound to,
	// and sock.laddr to the local address on the network that the socket
	// is linked to.
	//
	// The addresses may differ from the address passed as argument due to
	// random port assignment or wildcard address selection.
	bind(addr T, sock *socket[T]) wasi.Errno
	// Link attaches a socket to the network, using sock.proto and sock.laddr
	// to construct the network address that the socket is linked to.
	//
	// An error is returned if a socket was already linked to the same address.
	link(sock *socket[T]) wasi.Errno
	// Unlink detaches a socket from the network, using sock.proto and
	// sock.laddr to construct the network address that the socket is unlinked
	// from.
	//
	// The method is idempotent, no errors are returned if the socket wasn't
	// linked to the network.
	unlink(sock *socket[T]) wasi.Errno
	// Open an outbound connection to the given network address.
	dial(ctx context.Context, proto protocol, addr T) (net.Conn, wasi.Errno)
	// Open a listener accepting connections for the given network address.
	listen(ctx context.Context, proto protocol, addr T) (net.Listener, wasi.Errno)
	// Open a listening packet connection for the given network address.
	listenPacket(ctx context.Context, proto protocol, addr T) (net.PacketConn, wasi.Errno)
}

type ipnet[T inaddr[T]] struct {
	mutex            sync.Mutex
	ipnet            netip.Prefix
	sockets          map[netaddr[T]]*socket[T]
	dialFunc         func(context.Context, string, string) (net.Conn, error)
	listenFunc       func(context.Context, string, string) (net.Listener, error)
	listenPacketFunc func(context.Context, string, string) (net.PacketConn, error)
}

func (n *ipnet[T]) address() (sockaddr T) {
	return sockaddr.withAddr(n.ipnet.Addr())
}

func (n *ipnet[T]) contains(sockaddr T) bool {
	return n.ipnet.Contains(sockaddr.addrPort().Addr())
}

func (n *ipnet[T]) supports(proto protocol) bool {
	return proto == ip || proto == tcp || proto == udp
}

func (n *ipnet[T]) netaddr(networkAddress net.Addr) (sockaddr T, errno wasi.Errno) {
	var addrPort netip.AddrPort
	switch na := networkAddress.(type) {
	case *net.TCPAddr:
		addrPort = na.AddrPort()
	case *net.UDPAddr:
		addrPort = na.AddrPort()
	default:
		return sockaddr, wasi.EAFNOSUPPORT
	}
	var addr = addrPort.Addr()
	var port = addrPort.Port()
	sockaddr = sockaddr.withAddr(addr)
	sockaddr = sockaddr.withPort(int(port))
	return sockaddr, wasi.ESUCCESS
}

func (n *ipnet[T]) sockaddr(socketAddress wasi.SocketAddress) (sockaddr T, errno wasi.Errno) {
	var anyAddr T
	if anyAddr.family() != socketAddress.Family() {
		return sockaddr, wasi.EAFNOSUPPORT
	}
	var addr netip.Addr
	var port int
	switch sa := socketAddress.(type) {
	case *wasi.Inet4Address:
		addr = netip.AddrFrom4(sa.Addr)
		port = sa.Port
	case *wasi.Inet6Address:
		addr = netip.AddrFrom16(sa.Addr)
		port = sa.Port
	default:
		return sockaddr, wasi.EAFNOSUPPORT
	}
	if port < 0 || port > math.MaxUint16 {
		return sockaddr, wasi.EINVAL
	}
	sockaddr = sockaddr.withAddr(addr)
	sockaddr = sockaddr.withPort(port)
	return sockaddr, wasi.ESUCCESS
}

func (n *ipnet[T]) socket(addr netaddr[T]) *socket[T] {
	n.mutex.Lock()
	sock := n.sockets[addr]
	n.mutex.Unlock()
	return sock
}

func (n *ipnet[T]) bind(addr T, sock *socket[T]) wasi.Errno {
	laddr := netaddr[T]{sock.proto, addr}
	bound := netaddr[T]{sock.proto, addr}
	// IP networks have a specific address that can be used by the sockets,
	// they cannot bind to arbitrary endpoints.
	switch ipaddr := bound.sockaddr.addrPort().Addr(); {
	case ipaddr.IsUnspecified():
		bound.sockaddr = bound.sockaddr.withAddr(n.ipnet.Addr())
	case ipaddr != n.ipnet.Addr():
		return wasi.EADDRNOTAVAIL
	}

	addrPort := bound.sockaddr.addrPort()
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if addrPort.Port() != 0 {
		if _, used := n.sockets[bound]; used {
			// TODO:
			// - SO_REUSEADDR
			// - SO_REUSEPORT
			return wasi.EADDRINUSE
		}
	} else {
		var port int
		for port = 49152; port <= 65535; port++ {
			bound.sockaddr = bound.sockaddr.withPort(port)
			if _, used := n.sockets[bound]; !used {
				break
			}
		}
		if port == 65535 {
			return wasi.EADDRNOTAVAIL
		}
		laddr.sockaddr = laddr.sockaddr.withPort(port)
	}

	if n.sockets == nil {
		n.sockets = make(map[netaddr[T]]*socket[T])
	}

	sock.laddr = laddr.sockaddr
	sock.bound = bound.sockaddr
	n.sockets[bound] = sock
	return wasi.ESUCCESS
}

func (n *ipnet[T]) link(sock *socket[T]) wasi.Errno {
	var addr T
	return n.bind(addr.withAddr(n.ipnet.Addr()), sock)
}

func (n *ipnet[T]) unlink(sock *socket[T]) wasi.Errno {
	var addr netaddr[T]
	addr.protocol = sock.proto
	addr.sockaddr = sock.bound
	n.mutex.Lock()
	if n.sockets[addr] == sock {
		delete(n.sockets, addr)
	}
	n.mutex.Unlock()
	return wasi.ESUCCESS
}

func (n *ipnet[T]) dial(ctx context.Context, proto protocol, addr T) (net.Conn, wasi.Errno) {
	c, err := n.dialFunc(ctx, proto.String(), addr.String())
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return c, wasi.ESUCCESS
}

func (n *ipnet[T]) listen(ctx context.Context, proto protocol, addr T) (net.Listener, wasi.Errno) {
	network, address := listenNetworkAddress(proto, addr)
	l, err := n.listenFunc(ctx, network, address)
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return l, wasi.ESUCCESS
}

func (n *ipnet[T]) listenPacket(ctx context.Context, proto protocol, addr T) (net.PacketConn, wasi.Errno) {
	network, address := listenNetworkAddress(proto, addr)
	c, err := n.listenPacketFunc(ctx, network, address)
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return c, wasi.ESUCCESS
}

func listenNetworkAddress[T inaddr[T]](proto protocol, addr T) (network, address string) {
	return proto.String(), addr.unspecified().String()
}

type unixnet struct {
	mutex            sync.Mutex
	name             string
	sockets          map[netaddr[unix]]*socket[unix]
	dialFunc         func(context.Context, string, string) (net.Conn, error)
	listenFunc       func(context.Context, string, string) (net.Listener, error)
	listenPacketFunc func(context.Context, string, string) (net.PacketConn, error)
}

func (n *unixnet) address() unix {
	return unix{name: n.name}
}

func (n *unixnet) contains(addr unix) bool {
	return n.name == addr.name
}

func (n *unixnet) supports(proto protocol) bool {
	return proto == 0
}

func (n *unixnet) netaddr(addr net.Addr) (unix, wasi.Errno) {
	switch na := addr.(type) {
	case *net.UnixAddr:
		return unix{name: na.Name}, wasi.ESUCCESS
	default:
		return unix{}, wasi.EAFNOSUPPORT
	}
}

func (n *unixnet) sockaddr(addr wasi.SocketAddress) (unix, wasi.Errno) {
	switch sa := addr.(type) {
	case *wasi.UnixAddress:
		return unix{name: sa.Name}, wasi.ESUCCESS
	default:
		return unix{}, wasi.EAFNOSUPPORT
	}
}

func (n *unixnet) socket(addr netaddr[unix]) *socket[unix] {
	n.mutex.Lock()
	sock := n.sockets[addr]
	n.mutex.Unlock()
	return sock
}

func (n *unixnet) bind(addr unix, sock *socket[unix]) wasi.Errno {
	if addr.name != n.name {
		return wasi.EADDRNOTAVAIL
	}
	laddr := netaddr[unix]{sock.proto, addr}
	bound := netaddr[unix]{sock.proto, addr}

	n.mutex.Lock()
	defer n.mutex.Unlock()

	if _, exist := n.sockets[bound]; exist {
		return wasi.EADDRINUSE
	}
	if n.sockets == nil {
		n.sockets = make(map[netaddr[unix]]*socket[unix])
	}
	sock.laddr = laddr.sockaddr
	sock.bound = bound.sockaddr
	n.sockets[bound] = sock
	return wasi.ESUCCESS
}

func (n *unixnet) link(sock *socket[unix]) wasi.Errno {
	return wasi.ESUCCESS
}

func (n *unixnet) unlink(sock *socket[unix]) wasi.Errno {
	var addr netaddr[unix]
	addr.protocol = sock.proto
	addr.sockaddr = sock.laddr
	n.mutex.Lock()
	if n.sockets[addr] == sock {
		delete(n.sockets, addr)
	}
	n.mutex.Unlock()
	return wasi.ESUCCESS
}

func (n *unixnet) dial(ctx context.Context, _ protocol, addr unix) (net.Conn, wasi.Errno) {
	c, err := n.dialFunc(ctx, "unix", addr.String())
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return c, wasi.ESUCCESS
}

func (n *unixnet) listen(ctx context.Context, _ protocol, addr unix) (net.Listener, wasi.Errno) {
	l, err := n.listenFunc(ctx, "unix", addr.String())
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return l, wasi.ESUCCESS
}

func (n *unixnet) listenPacket(ctx context.Context, _ protocol, addr unix) (net.PacketConn, wasi.Errno) {
	c, err := n.listenPacketFunc(ctx, "unixgram", addr.String())
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return c, wasi.ESUCCESS
}

var (
	_ network[ipv4] = (*ipnet[ipv4])(nil)
	_ network[ipv6] = (*ipnet[ipv6])(nil)
	_ network[unix] = (*unixnet)(nil)
)
