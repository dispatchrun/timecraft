package sandbox

import (
	"context"
	"net"
	"net/netip"
	"sync"
	"time"

	"github.com/stealthrocket/wasi-go"
)

// Connect opens a connection to a listening socket on the guest module network.
//
// This function has a signature that matches the one commonly used in the
// Go standard library as a hook to customize how and where network connections
// are estalibshed. The intent is for this function to be used when the host
// needs to establish a connection to the guest, maybe indirectly such as using
// a http.Transport and setting this method as the transport's dial function.
func (s *System) Connect(ctx context.Context, network, address string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6":
		addrPort, err := netip.ParseAddrPort(address)
		if err != nil {
			return nil, &net.ParseError{Type: "connect address", Text: address}
		}
		addr := addrPort.Addr()
		port := addrPort.Port()
		if port == 0 {
			return nil, &net.AddrError{Err: "missing port in connect address", Addr: address}
		}
		if addr.Is4() {
			return s.ipv4.connect(ctx, netaddr[ipv4]{
				protocol: tcp,
				sockaddr: ipv4{
					Port: int(port),
					Addr: addr.As4(),
				},
			})
		} else {
			return s.ipv6.connect(ctx, netaddr[ipv6]{
				protocol: tcp,
				sockaddr: ipv6{
					Port: int(port),
					Addr: addr.As16(),
				},
			})
		}
	case "unix":
		return s.unix.connect(ctx, netaddr[unix]{
			sockaddr: unix{
				Name: address,
			},
		})
	default:
		return nil, &net.OpError{Op: "connect", Net: network, Err: net.UnknownNetworkError(network)}
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
		addrPort, err := netip.ParseAddrPort(address)
		if err != nil {
			return nil, &net.ParseError{Type: "listen address", Text: address}
		}
		addr := addrPort.Addr()
		port := addrPort.Port()
		if addr.Is4() {
			return s.ipv4.listen(ctx, s.lock, netaddr[ipv4]{
				protocol: tcp,
				sockaddr: ipv4{
					Port: int(port),
					Addr: addr.As4(),
				},
			})
		} else {
			return s.ipv6.listen(ctx, s.lock, netaddr[ipv6]{
				protocol: tcp,
				sockaddr: ipv6{
					Port: int(port),
					Addr: addr.As16(),
				},
			})
		}
	case "unix":
		return s.unix.listen(ctx, s.lock, netaddr[unix]{
			sockaddr: unix{
				Name: address,
			},
		})
	default:
		return nil, &net.OpError{Op: "listen", Net: network, Err: net.UnknownNetworkError(network)}
	}
}

type sockaddr interface {
	comparable
	family() wasi.ProtocolFamily
	sockaddr() wasi.SocketAddress
	netAddr(protocol) net.Addr
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

type ipv4 wasi.Inet4Address

func (inaddr ipv4) family() wasi.ProtocolFamily {
	return wasi.InetFamily
}

func (inaddr ipv4) sockaddr() wasi.SocketAddress {
	return (*wasi.Inet4Address)(&inaddr)
}

func (inaddr ipv4) netAddr(proto protocol) net.Addr {
	return makeIPNetAddr(proto, net.IP(inaddr.Addr[:]), inaddr.Port)
}

type ipv6 wasi.Inet6Address

func (inaddr ipv6) family() wasi.ProtocolFamily {
	return wasi.Inet6Family
}

func (inaddr ipv6) sockaddr() wasi.SocketAddress {
	return (*wasi.Inet6Address)(&inaddr)
}

func (inaddr ipv6) netAddr(proto protocol) net.Addr {
	return makeIPNetAddr(proto, net.IP(inaddr.Addr[:]), inaddr.Port)
}

type unix wasi.UnixAddress

func (unaddr unix) family() wasi.ProtocolFamily {
	return wasi.UnixFamily
}

func (unaddr unix) sockaddr() wasi.SocketAddress {
	return (*wasi.UnixAddress)(&unaddr)
}

func (unaddr unix) netAddr(protocol) net.Addr {
	return &net.UnixAddr{Net: "unix", Name: unaddr.Name}
}

type protocol wasi.Protocol

const (
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

type network[T sockaddr] struct {
	mutex   sync.Mutex
	sockets map[netaddr[T]]*socket[T]
}

func (n *network[T]) open(pmu *sync.Mutex, proto protocol) *socket[T] {
	return &socket[T]{
		net:   n,
		proto: proto,
		pipe:  makePipe(pmu),
	}
}

func (n *network[T]) connect(ctx context.Context, addr netaddr[T]) (net.Conn, error) {
	n.mutex.Lock()
	server, ok := n.sockets[addr]
	n.mutex.Unlock()
	if !ok {
		netAddr := addr.netAddr()
		return nil, &net.OpError{Op: "connect", Net: netAddr.Network(), Addr: netAddr, Err: wasi.ECONNREFUSED}
	}
	return server.connect(ctx)
}

func (n *network[T]) listen(ctx context.Context, pmu *sync.Mutex, addr netaddr[T]) (net.Listener, error) {
	sock := n.open(pmu, addr.protocol)
	sock.listen()
	if errno := n.bind(addr, sock); errno != wasi.ESUCCESS {
		netAddr := addr.netAddr()
		return nil, &net.OpError{Op: "listen", Net: netAddr.Network(), Addr: netAddr, Err: errno}
	}
	return listener[T]{sock}, nil
}

func (n *network[T]) bind(addr netaddr[T], sock *socket[T]) wasi.Errno {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if _, exist := n.sockets[addr]; exist {
		// TODO:
		// - SO_REUSEADDR
		// - SO_REUSEPORT
		return wasi.EADDRINUSE
	}
	if n.sockets == nil {
		n.sockets = make(map[netaddr[T]]*socket[T])
	}
	sock.proto = addr.protocol
	sock.laddr = addr.sockaddr
	n.sockets[addr] = sock
	return wasi.ESUCCESS
}

func (n *network[T]) unlink(addr netaddr[T], sock *socket[T]) {
	n.mutex.Lock()
	if n.sockets[addr] == sock {
		delete(n.sockets, addr)
	}
	n.mutex.Unlock()
}

type listener[T sockaddr] struct {
	socket *socket[T]
}

func (l listener[T]) Close() error {
	return l.socket.Close()
}

func (l listener[T]) Addr() net.Addr {
	return l.socket.LocalAddr()
}

func (l listener[T]) Accept() (net.Conn, error) {
	select {
	case conn := <-l.socket.accept:
		return conn, nil
	case <-l.socket.done:
		return nil, net.ErrClosed
	}
}

type socket[T sockaddr] struct {
	defaultFile
	net    *network[T]
	proto  protocol
	raddr  T
	laddr  T
	ready  event // ready when the socket is accepting a new connection
	accept chan *socket[T]
	pipe
}

func (s *socket[T]) close() {
	s.net.unlink(netaddr[T]{protocol: s.proto, sockaddr: s.laddr}, s)
	s.pipe.close()
}

func (s *socket[T]) connect(ctx context.Context) (net.Conn, error) {
	if s.accept == nil {
		return nil, &net.OpError{Op: "connect", Net: s.proto.String(), Err: wasi.EISCONN}
	}
	conn := &socket[T]{
		proto: s.proto,
		raddr: s.laddr,
		// TODO: laddr
	}
	s.ready.trigger(s.pmu)
	select {
	case s.accept <- conn:
		return conn, nil
	case <-s.done:
		return nil, net.ErrClosed
	case <-ctx.Done():
		return nil, context.Cause(ctx)
	}
}

func (s *socket[T]) listen() {
	s.accept = make(chan *socket[T])
}

func (s *socket[T]) Hook(ev wasi.EventType, ch chan<- struct{}) {
	switch {
	case s.accept == nil:
		s.pipe.Hook(ev, ch)
	case ev == wasi.FDReadEvent:
		s.ready.hook(ch)
	}
}

func (s *socket[T]) Poll(ev wasi.EventType) bool {
	switch {
	case s.accept == nil:
		return s.pipe.Poll(ev)
	case ev == wasi.FDReadEvent:
		return s.ready.poll()
	default:
		return false
	}
}

func (s *socket[T]) SockListen(ctx context.Context, _ int) wasi.Errno {
	// TODO: we ignore the backlog parameter for now because it complicates a lot
	// of the concurrent logic to havea buffered channel for accept.
	if s.accept == nil {
		var zero T
		if s.laddr == zero {
			return wasi.EDESTADDRREQ
		}
		s.listen()
	}
	return wasi.ESUCCESS
}

func (s *socket[T]) SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno) {
	select {
	case conn := <-s.accept:
		return conn, wasi.ESUCCESS
	case <-s.done:
		return nil, wasi.EBADF
	case <-ctx.Done():
		return nil, wasi.MakeErrno(ctx.Err())
	}
}

func (s *socket[T]) SockRecv(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	return 0, 0, wasi.ENOSYS // TODO: implement recv
}

func (s *socket[T]) SockSend(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOSYS // TODO: implement send
}

func (s *socket[T]) SockSendTo(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOSYS // TODO: implement sentto
}

func (s *socket[T]) SockRecvFrom(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	return 0, 0, nil, wasi.ENOSYS // TODO: implement recvfrom
}

func (s *socket[T]) SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return s.laddr.sockaddr(), wasi.ESUCCESS
}

func (s *socket[T]) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return s.laddr.sockaddr(), wasi.ESUCCESS
}

func (s *socket[T]) SockGetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	return nil, wasi.ENOSYS // TODO: support socket options
}

func (s *socket[T]) SockSetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	return wasi.ENOSYS // TODO: support socket options
}

func (s *socket[T]) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	return wasi.ENOSYS // TOOD: support socket shutdown
}

func (s *socket[T]) FDClose(ctx context.Context) wasi.Errno {
	s.close()
	return s.pipe.FDClose(ctx)
}

func (s *socket[T]) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return s.pipe.FDStatSetFlags(ctx, flags)
}

func (s *socket[T]) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.pipe.FDRead(ctx, iovs)
}

func (s *socket[T]) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.pipe.FDWrite(ctx, iovs)
}

func (s *socket[T]) Close() error {
	s.close()
	return nil
}

func (s *socket[T]) LocalAddr() net.Addr {
	return s.laddr.netAddr(s.proto)
}

func (s *socket[T]) RemoteAddr() net.Addr {
	return s.raddr.netAddr(s.proto)
}

func (s *socket[T]) SetDeadline(t time.Time) error {
	raddr, laddr := s.RemoteAddr(), s.LocalAddr()
	return &net.OpError{Op: "SetDeadline", Net: raddr.Network(), Source: laddr, Addr: raddr, Err: wasi.ENOSYS}
}

func (s *socket[T]) SetReadDeadline(t time.Time) error {
	raddr, laddr := s.RemoteAddr(), s.LocalAddr()
	return &net.OpError{Op: "SetReadDeadline", Net: raddr.Network(), Source: laddr, Addr: raddr, Err: wasi.ENOSYS}
}

func (s *socket[T]) SetWriteDeadline(t time.Time) error {
	raddr, laddr := s.RemoteAddr(), s.LocalAddr()
	return &net.OpError{Op: "SetWriteDeadline", Net: raddr.Network(), Source: laddr, Addr: raddr, Err: wasi.ENOSYS}
}
