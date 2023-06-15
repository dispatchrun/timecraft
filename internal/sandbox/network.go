package sandbox

import (
	"context"
	"errors"
	"io"
	"net"
	"net/netip"
	"os"
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
			return connect(ctx, &s.ipv4, netaddr[ipv4]{
				protocol: tcp,
				sockaddr: ipv4{
					Port: int(port),
					Addr: addr.As4(),
				},
			})
		} else {
			return connect(ctx, &s.ipv6, netaddr[ipv6]{
				protocol: tcp,
				sockaddr: ipv6{
					Port: int(port),
					Addr: addr.As16(),
				},
			})
		}

	case "unix":
		return connect(ctx, &s.unix, netaddr[unix]{
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
		h, p, err := net.SplitHostPort(address)
		if err != nil {
			return nil, &net.ParseError{Type: "listen address", Text: address}
		}
		// Allow omitting the address to let the system select the best match.
		if h == "" {
			if network == "tcp6" {
				h = "[::]"
			} else {
				h = "0.0.0.0"
			}
		}

		addrPort, err := netip.ParseAddrPort(net.JoinHostPort(h, p))
		if err != nil {
			return nil, &net.ParseError{Type: "listen address", Text: address}
		}

		addr := addrPort.Addr()
		port := addrPort.Port()
		if addr.Is4() {
			return listen(&s.ipv4, s.lock, netaddr[ipv4]{
				protocol: tcp,
				sockaddr: ipv4{
					Port: int(port),
					Addr: addr.As4(),
				},
			})
		} else {
			return listen(&s.ipv6, s.lock, netaddr[ipv6]{
				protocol: tcp,
				sockaddr: ipv6{
					Port: int(port),
					Addr: addr.As16(),
				},
			})
		}

	case "unix":
		return listen(&s.unix, s.lock, netaddr[unix]{
			sockaddr: unix{
				Name: address,
			},
		})
	default:
		return nil, &net.OpError{Op: "listen", Net: network, Err: net.UnknownNetworkError(network)}
	}
}

type sockaddr interface {
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
	sockaddr
}

type ipv4 wasi.Inet4Address

func (inaddr ipv4) addrPort() netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom4(inaddr.Addr), uint16(inaddr.Port))
}

func (inaddr ipv4) withAddr(addr netip.Addr) ipv4 {
	return ipv4{Addr: addr.As4(), Port: inaddr.Port}
}

func (inaddr ipv4) withPort(port int) ipv4 {
	return ipv4{Addr: inaddr.Addr, Port: port}
}

func (inaddr ipv4) sockAddr() wasi.SocketAddress {
	return (*wasi.Inet4Address)(&inaddr)
}

func (inaddr ipv4) netAddr(proto protocol) net.Addr {
	return makeIPNetAddr(proto, net.IP(inaddr.Addr[:]), inaddr.Port)
}

type ipv6 wasi.Inet6Address

func (inaddr ipv6) addrPort() netip.AddrPort {
	return netip.AddrPortFrom(netip.AddrFrom16(inaddr.Addr), uint16(inaddr.Port))
}

func (inaddr ipv6) withAddr(addr netip.Addr) ipv6 {
	return ipv6{Addr: addr.As16(), Port: inaddr.Port}
}

func (inaddr ipv6) withPort(port int) ipv6 {
	return ipv6{Addr: inaddr.Addr, Port: port}
}

func (inaddr ipv6) sockAddr() wasi.SocketAddress {
	return (*wasi.Inet6Address)(&inaddr)
}

func (inaddr ipv6) netAddr(proto protocol) net.Addr {
	return makeIPNetAddr(proto, net.IP(inaddr.Addr[:]), inaddr.Port)
}

type unix wasi.UnixAddress

func (unaddr unix) sockAddr() wasi.SocketAddress {
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

type network[T sockaddr] interface {
	socket(addr netaddr[T]) *socket[T]
	bind(addr netaddr[T], sock *socket[T]) wasi.Errno
	link(sock *socket[T]) wasi.Errno
	unlink(sock *socket[T]) wasi.Errno
}

func connect[N network[T], T sockaddr](ctx context.Context, n N, addr netaddr[T]) (net.Conn, error) {
	sock := n.socket(addr)
	if sock == nil {
		netAddr := addr.netAddr()
		return nil, &net.OpError{Op: "connect", Net: netAddr.Network(), Addr: netAddr, Err: wasi.ECONNREFUSED}
	}
	return sock.connect(ctx)
}

func listen[N network[T], T sockaddr](n N, lock *sync.Mutex, addr netaddr[T]) (net.Listener, error) {
	sock := newSocket[T](n, lock, addr.protocol)
	sock.listen()
	if errno := n.bind(addr, sock); errno != wasi.ESUCCESS {
		netAddr := addr.netAddr()
		return nil, &net.OpError{Op: "listen", Net: netAddr.Network(), Addr: netAddr, Err: errno}
	}
	return &listener[T]{socket: sock, addr: sock.laddr.netAddr(sock.proto)}, nil
}

type listener[T sockaddr] struct {
	socket *socket[T]
	addr   net.Addr
}

func (l *listener[T]) Close() error {
	l.socket.close()
	return nil
}

func (l *listener[T]) Addr() net.Addr {
	return l.addr
}

func (l *listener[T]) Accept() (net.Conn, error) {
	select {
	case socket := <-l.socket.accept:
		if socket.host {
			return newGuestConn(socket), nil
		} else {
			return newHostConn(socket), nil
		}
	case <-l.socket.recv.done:
		return nil, net.ErrClosed
	}
}

type ipnet[T inaddr[T]] struct {
	mutex   sync.Mutex
	address netip.Addr
	sockets map[netaddr[T]]*socket[T]
}

func (n *ipnet[T]) socket(addr netaddr[T]) *socket[T] {
	n.mutex.Lock()
	sock := n.sockets[addr]
	n.mutex.Unlock()
	return sock
}

func (n *ipnet[T]) bind(addr netaddr[T], sock *socket[T]) wasi.Errno {
	// IP networks have a specific address that can be used by the sockets,
	// they cannot bind to arbitrary endpoints.
	switch ipaddr := addr.sockaddr.addrPort().Addr(); {
	case ipaddr.IsUnspecified():
		addr.sockaddr = addr.sockaddr.withAddr(n.address)
	case ipaddr != n.address:
		return wasi.EADDRNOTAVAIL
	}

	addrPort := addr.sockaddr.addrPort()
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if addrPort.Port() != 0 {
		if _, used := n.sockets[addr]; used {
			// TODO:
			// - SO_REUSEADDR
			// - SO_REUSEPORT
			return wasi.EADDRINUSE
		}
	} else {
		var port int
		for port = 49152; port <= 65535; port++ {
			addr.sockaddr = addr.sockaddr.withPort(port)
			if _, used := n.sockets[addr]; !used {
				break
			}
		}
		if port > 65535 {
			return wasi.EADDRNOTAVAIL
		}
	}

	if n.sockets == nil {
		n.sockets = make(map[netaddr[T]]*socket[T])
	}

	sock.laddr = addr.sockaddr
	n.sockets[addr] = sock
	return wasi.ESUCCESS
}

func (n *ipnet[T]) link(sock *socket[T]) wasi.Errno {
	var addr netaddr[T]
	addr.protocol = sock.proto
	addr.sockaddr = addr.sockaddr.withAddr(n.address)
	return n.bind(addr, sock)
}

func (n *ipnet[T]) unlink(sock *socket[T]) wasi.Errno {
	var addr netaddr[T]
	addr.protocol = sock.proto
	addr.sockaddr = sock.laddr
	n.mutex.Lock()
	if n.sockets[addr] == sock {
		delete(n.sockets, addr)
	}
	n.mutex.Unlock()
	return wasi.ESUCCESS
}

type unixnet struct {
	mutex   sync.Mutex
	name    string
	sockets map[netaddr[unix]]*socket[unix]
}

func (n *unixnet) socket(addr netaddr[unix]) *socket[unix] {
	n.mutex.Lock()
	sock := n.sockets[addr]
	n.mutex.Unlock()
	return sock
}

func (n *unixnet) bind(addr netaddr[unix], sock *socket[unix]) wasi.Errno {
	if addr.sockaddr.Name != n.name {
		return wasi.EADDRNOTAVAIL
	}
	n.mutex.Lock()
	defer n.mutex.Unlock()

	if _, exist := n.sockets[addr]; exist {
		return wasi.EADDRINUSE
	}
	if n.sockets == nil {
		n.sockets = make(map[netaddr[unix]]*socket[unix])
	}
	sock.laddr = addr.sockaddr
	n.sockets[addr] = sock
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

var (
	_ network[ipv4] = (*ipnet[ipv4])(nil)
	_ network[ipv6] = (*ipnet[ipv6])(nil)
	_ network[unix] = (*unixnet)(nil)
)

type socket[T sockaddr] struct {
	defaultFile
	net   network[T]
	proto protocol
	raddr T
	laddr T
	// For listening sockets, this event and channel are used to pass
	// connections between the two ends of the network.
	accept chan *socket[T]
	// Connected sockets have a bidirectional pipe that links between the two
	// ends of the socket pair.
	send *pipe // socket send end (guest side)
	recv *pipe // socket recv end (guest side)
	// This field is set to true if the socket was created by the host, which
	// allows listeners to detect if they are accepting connections from the
	// guest, and construct the right connection type.
	host bool
}

func newSocket[T sockaddr](net network[T], lock *sync.Mutex, proto protocol) *socket[T] {
	return &socket[T]{
		net:   net,
		proto: proto,
		send:  newPipe(lock),
		recv:  newPipe(lock),
	}
}

func (s *socket[T]) close() {
	_ = s.net.unlink(s)
	s.recv.close()
	s.send.close()
}

func (s *socket[T]) connect(ctx context.Context) (net.Conn, error) {
	if s.accept == nil {
		return nil, &net.OpError{Op: "connect", Net: s.proto.String(), Err: wasi.EISCONN}
	}
	// The lock is the same on both the send and receive ends.
	lock := s.send.lock
	conn := &socket[T]{
		net:   s.net,
		proto: s.proto,
		send:  newPipe(lock),
		recv:  newPipe(lock),
		raddr: s.laddr,
		host:  true,
	}
	if errno := s.net.link(conn); errno != wasi.ESUCCESS {
		return nil, errno
	}
	conn.laddr, conn.raddr = conn.raddr, conn.laddr

	s.recv.ev.trigger(s.recv.lock)
	select {
	case s.accept <- conn:
		return newHostConn(conn), nil
	case <-s.recv.done:
		_ = s.net.unlink(conn)
		return nil, net.ErrClosed
	case <-ctx.Done():
		_ = s.net.unlink(conn)
		return nil, context.Cause(ctx)
	}
}

func (s *socket[T]) listen() {
	s.accept = make(chan *socket[T])
}

func (s *socket[T]) FDHook(ev wasi.EventType, ch chan<- struct{}) {
	switch ev {
	case wasi.FDReadEvent:
		s.recv.ev.hook(ch)
	case wasi.FDWriteEvent:
		s.send.ev.hook(ch)
	}
}

func (s *socket[T]) FDPoll(ev wasi.EventType) bool {
	switch ev {
	case wasi.FDReadEvent:
		return s.recv.ev.poll()
	case wasi.FDWriteEvent:
		return s.send.ev.poll()
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
		s.accept = make(chan *socket[T])
	}
	return wasi.ESUCCESS
}

func (s *socket[T]) SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno) {
	select {
	case sock := <-s.accept:
		return sock, wasi.ESUCCESS
	case <-s.recv.done:
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
	return s.laddr.sockAddr(), wasi.ESUCCESS
}

func (s *socket[T]) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return s.laddr.sockAddr(), wasi.ESUCCESS
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
	s.send.FDClose(ctx)
	s.recv.FDClose(ctx)
	return wasi.ESUCCESS
}

func (s *socket[T]) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	_ = s.send.FDStatSetFlags(ctx, flags)
	_ = s.recv.FDStatSetFlags(ctx, flags)
	return wasi.ESUCCESS
}

func (s *socket[T]) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.recv.ch.read(ctx, iovs, s.recv.flags, &s.recv.ev, nil, s.recv.done)
}

func (s *socket[T]) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.send.ch.write(ctx, iovs, s.send.flags, &s.send.ev, nil, s.send.done)
}

type hostConn[T sockaddr] struct {
	socket    *socket[T]
	laddr     net.Addr
	raddr     net.Addr
	rdeadline deadline
	wdeadline deadline
}

func newHostConn[T sockaddr](socket *socket[T]) *hostConn[T] {
	return &hostConn[T]{
		socket:    socket,
		laddr:     socket.raddr.netAddr(socket.proto),
		raddr:     socket.laddr.netAddr(socket.proto),
		rdeadline: makeDeadline(),
		wdeadline: makeDeadline(),
	}
}

func (c *hostConn[T]) Close() error {
	c.socket.close()
	c.rdeadline.set(time.Time{})
	c.wdeadline.set(time.Time{})
	return nil
}

func (c *hostConn[T]) Read(b []byte) (int, error) {
	pipe := c.socket.send
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if c.rdeadline.expired() {
		return 0, os.ErrDeadlineExceeded
	}

	pipe.ev.trigger(pipe.lock)
	select {
	case pipe.ch <- b:
		return len(b) - len(<-pipe.ch), nil
	case <-c.rdeadline.ch:
		return 0, os.ErrDeadlineExceeded
	case <-pipe.done:
		return 0, io.EOF
	}
}

func (c *hostConn[T]) Write(b []byte) (n int, err error) {
	pipe := c.socket.recv
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if c.wdeadline.expired() {
		return 0, os.ErrDeadlineExceeded
	}

	for n < len(b) {
		pipe.ev.trigger(pipe.lock)
		select {
		case pipe.ch <- b[n:]:
			n = len(b) - len(<-pipe.ch)
		case <-c.wdeadline.ch:
			return n, os.ErrDeadlineExceeded
		case <-pipe.done:
			return n, io.ErrClosedPipe
		}
	}
	return n, nil
}

func (c *hostConn[T]) LocalAddr() net.Addr {
	return c.laddr
}

func (c *hostConn[T]) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *hostConn[T]) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}

func (c *hostConn[T]) SetReadDeadline(t time.Time) error {
	select {
	case <-c.socket.send.done:
		return net.ErrClosed
	default:
		c.rdeadline.set(t)
		return nil
	}
}

func (c *hostConn[T]) SetWriteDeadline(t time.Time) error {
	select {
	case <-c.socket.recv.done:
		return net.ErrClosed
	default:
		c.wdeadline.set(t)
		return nil
	}
}

type guestConn[T sockaddr] struct {
	socket    *socket[T]
	laddr     net.Addr
	raddr     net.Addr
	rdeadline deadline
	wdeadline deadline
}

func newGuestConn[T sockaddr](socket *socket[T]) *guestConn[T] {
	return &guestConn[T]{
		socket:    socket,
		laddr:     socket.laddr.netAddr(socket.proto),
		raddr:     socket.raddr.netAddr(socket.proto),
		rdeadline: makeDeadline(),
		wdeadline: makeDeadline(),
	}
}

func (c *guestConn[T]) Close() error {
	c.socket.close()
	c.rdeadline.set(time.Time{})
	c.wdeadline.set(time.Time{})
	return nil
}

func (c *guestConn[T]) Read(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	if c.rdeadline.expired() {
		return 0, os.ErrDeadlineExceeded
	}
	ctx := context.Background()
	pipe := c.socket.recv
	timeout := c.rdeadline.ch
	iovs := []wasi.IOVec{b}
	size, errno := pipe.ch.read(ctx, iovs, 0, &pipe.ev, timeout, pipe.done)
	switch errno {
	case wasi.ESUCCESS:
		if size == 0 {
			return 0, io.EOF
		}
		return int(size), nil
	case wasi.ETIMEDOUT:
		return int(size), os.ErrDeadlineExceeded
	case wasi.EBADF:
		return int(size), net.ErrClosed
	default:
		return int(size), errno
	}
}

func (c *guestConn[T]) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}
	if c.wdeadline.expired() {
		return 0, os.ErrDeadlineExceeded
	}
	ctx := context.Background()
	pipe := c.socket.send
	timeout := c.wdeadline.ch
	for n < len(b) {
		iovs := []wasi.IOVec{b[n:]}
		size, errno := pipe.ch.write(ctx, iovs, 0, &pipe.ev, timeout, pipe.done)
		n += int(size)
		switch errno {
		case wasi.ESUCCESS:
		case wasi.ETIMEDOUT:
			return n, os.ErrDeadlineExceeded
		case wasi.EBADF:
			return n, net.ErrClosed
		default:
			return n, errno
		}
	}
	return n, nil
}

func (c *guestConn[T]) LocalAddr() net.Addr {
	return c.laddr
}

func (c *guestConn[T]) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *guestConn[T]) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}

func (c *guestConn[T]) SetReadDeadline(t time.Time) error {
	select {
	case <-c.socket.recv.done:
		return net.ErrClosed
	default:
		c.rdeadline.set(t)
		return nil
	}
}

func (c *guestConn[T]) SetWriteDeadline(t time.Time) error {
	select {
	case <-c.socket.send.done:
		return net.ErrClosed
	default:
		c.wdeadline.set(t)
		return nil
	}
}

type deadline struct {
	mu sync.Mutex
	ts time.Time
	tm *time.Timer
	ch chan struct{}
}

func makeDeadline() deadline {
	return deadline{ch: make(chan struct{}, 1)}
}

func (d *deadline) expired() bool {
	d.mu.Lock()
	ts := d.ts
	d.mu.Unlock()
	return !ts.IsZero() && !ts.After(time.Now())
}

func (d *deadline) set(t time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.ts = t
	d.clear()

	if t.IsZero() {
		if d.tm != nil {
			d.tm.Stop()
		}
		return
	}

	if timeout := time.Until(t); timeout <= 0 {
		if d.tm != nil {
			d.tm.Stop()
		}
		d.signal()
	} else if d.tm != nil {
		d.tm.Reset(timeout)
	} else {
		d.tm = time.AfterFunc(timeout, d.signal)
	}
}

func (d *deadline) clear() {
	select {
	case <-d.ch:
	default:
	}
}

func (d *deadline) signal() {
	select {
	case d.ch <- struct{}{}:
	default:
	}
}
