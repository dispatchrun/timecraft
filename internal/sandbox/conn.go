package sandbox

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stealthrocket/wasi-go"
)

type conn[T sockaddr] struct {
	socket *socket[T]
	laddr  T
	raddr  T

	rmu       sync.Mutex
	rev       *event
	rbuf      *sockbuf[T]
	rdeadline deadline
	rpoll     chan struct{}

	wmu       sync.Mutex
	wev       *event
	wbuf      *sockbuf[T]
	wdeadline deadline
	wpoll     chan struct{}

	done chan struct{}
	once sync.Once
}

func newConn[T sockaddr](socket *socket[T]) *conn[T] {
	return &conn[T]{
		socket:    socket,
		rdeadline: makeDeadline(),
		wdeadline: makeDeadline(),
		rpoll:     make(chan struct{}, 1),
		wpoll:     make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
}

func newHostConn[T sockaddr](socket *socket[T]) *conn[T] {
	c := newConn(socket)
	c.laddr = socket.raddr
	c.raddr = socket.laddr
	c.rbuf = socket.wbuf
	c.wbuf = socket.rbuf
	c.rev = &socket.wbuf.rev
	c.wev = &socket.rbuf.wev
	return c
}

func newGuestConn[T sockaddr](socket *socket[T]) *conn[T] {
	c := newConn(socket)
	c.laddr = socket.laddr
	c.raddr = socket.raddr
	c.rbuf = socket.rbuf
	c.wbuf = socket.wbuf
	c.rev = &socket.rbuf.rev
	c.wev = &socket.wbuf.wev
	return c
}

func (c *conn[T]) String() string {
	return fmt.Sprintf("%s->%s", c.raddr, c.laddr)
}

func (c *conn[T]) Close() error {
	c.socket.close()
	c.rdeadline.set(time.Time{})
	c.wdeadline.set(time.Time{})
	c.once.Do(func() { close(c.done) })
	return nil
}

func (c *conn[T]) CloseRead() error {
	c.rbuf.close()
	return nil
}

func (c *conn[T]) CloseWrite() error {
	c.wbuf.close()
	return nil
}

func (c *conn[T]) Read(b []byte) (int, error) {
	c.rmu.Lock() // serialize reads
	defer c.rmu.Unlock()

	for {
		if c.rdeadline.expired() {
			return 0, c.newError("read", os.ErrDeadlineExceeded)
		}

		n, _, _, errno := c.rbuf.recv([]wasi.IOVec{b}, 0)
		if errno == wasi.ESUCCESS {
			if n == 0 {
				return 0, io.EOF
			}
			return int(n), nil
		}
		if errno != wasi.EAGAIN {
			return int(n), c.newError("read", errno.Syscall())
		}

		var ready bool
		c.rev.synchronize(func() { ready = c.rev.poll(c.rpoll) })

		if !ready {
			select {
			case <-c.rpoll:
			case <-c.done:
				return 0, io.EOF
			case <-c.rdeadline.channel():
				return 0, c.newError("read", os.ErrDeadlineExceeded)
			}
		}
	}
}

func (c *conn[T]) Write(b []byte) (int, error) {
	c.wmu.Lock() // serialize writes
	defer c.wmu.Unlock()

	var n int
	for {
		if c.wdeadline.expired() {
			return n, c.newError("write", os.ErrDeadlineExceeded)
		}

		r, errno := c.wbuf.send([]wasi.IOVec{b}, c.laddr)
		if rn := int(int32(r)); rn > 0 {
			n += rn
			b = b[rn:]
		}
		if errno == wasi.ESUCCESS {
			if len(b) != 0 {
				continue
			}
			return n, nil
		}
		if errno != wasi.EAGAIN {
			return n, c.newError("write", errno.Syscall())
		}

		var ready bool
		c.wev.synchronize(func() { ready = c.wev.poll(c.wpoll) })

		if !ready {
			select {
			case <-c.wpoll:
			case <-c.done:
				return n, io.EOF
			case <-c.wdeadline.channel():
				return n, c.newError("write", os.ErrDeadlineExceeded)
			}
		}
	}
}

func (c *conn[T]) LocalAddr() net.Addr {
	return c.laddr.netAddr(c.socket.proto)
}

func (c *conn[T]) RemoteAddr() net.Addr {
	return c.raddr.netAddr(c.socket.proto)
}

func (c *conn[T]) SetDeadline(t time.Time) error {
	select {
	case <-c.done:
		return c.newError("set", net.ErrClosed)
	default:
		c.rdeadline.set(t)
		c.wdeadline.set(t)
		return nil
	}
}

func (c *conn[T]) SetReadDeadline(t time.Time) error {
	select {
	case <-c.done:
		return c.newError("set", net.ErrClosed)
	default:
		c.rdeadline.set(t)
		return nil
	}
}

func (c *conn[T]) SetWriteDeadline(t time.Time) error {
	select {
	case <-c.done:
		return c.newError("set", net.ErrClosed)
	default:
		c.wdeadline.set(t)
		return nil
	}
}

func (c *conn[T]) newError(op string, err error) error {
	return newConnError(op, c.LocalAddr(), c.RemoteAddr(), err)
}

var (
	_ net.Conn = (*conn[ipv4])(nil)
)

type packetConn[T sockaddr] struct {
	socket *socket[T]

	rmu       sync.Mutex
	rdeadline deadline
	rpoll     chan struct{}

	wmu       sync.Mutex
	wdeadline deadline
	wpoll     chan struct{}

	done chan struct{}
	once sync.Once
}

func newPacketConn[T sockaddr](socket *socket[T]) *packetConn[T] {
	return &packetConn[T]{
		socket:    socket,
		rdeadline: makeDeadline(),
		wdeadline: makeDeadline(),
		rpoll:     make(chan struct{}, 1),
		wpoll:     make(chan struct{}, 1),
		done:      make(chan struct{}),
	}
}

func (c *packetConn[T]) String() string {
	return fmt.Sprintf("%s->%s", c.socket.raddr, c.socket.laddr)
}

func (c *packetConn[T]) Close() error {
	c.socket.close()
	c.rdeadline.set(time.Time{})
	c.wdeadline.set(time.Time{})
	c.once.Do(func() { close(c.done) })
	return nil
}

func (c *packetConn[T]) CloseRead() error {
	c.socket.rbuf.close()
	return nil
}

func (c *packetConn[T]) CloseWrite() error {
	c.socket.wbuf.close()
	return nil
}

func (c *packetConn[T]) Read(b []byte) (int, error) {
	n, _, err := c.readFrom(b)
	return n, err
}

func (c *packetConn[T]) ReadFrom(b []byte) (int, net.Addr, error) {
	n, addr, err := c.readFrom(b)
	if err != nil {
		return n, nil, err
	}
	return n, addr.netAddr(c.socket.proto), nil
}

func (c *packetConn[T]) readFrom(b []byte) (int, T, error) {
	c.rmu.Lock() // serialize reads
	defer c.rmu.Unlock()

	var zero T
	select {
	case <-c.done:
		return 0, zero, io.EOF
	default:
	}

	for {
		if c.rdeadline.expired() {
			return 0, zero, c.newError("read", os.ErrDeadlineExceeded)
		}

		n, _, addr, errno := c.socket.rbuf.recvmsg([]wasi.IOVec{b}, 0)
		if errno == wasi.ESUCCESS {
			if n == 0 {
				return 0, zero, io.EOF
			}
			if c.socket.raddr != zero && c.socket.raddr != addr {
				continue
			}
			return int(n), addr, nil
		}
		if errno != wasi.EAGAIN {
			return int(n), zero, c.newError("read", errno.Syscall())
		}

		var ready bool
		c.socket.rev.synchronize(func() { ready = c.socket.rev.poll(c.rpoll) })

		if !ready {
			select {
			case <-c.rpoll:
			case <-c.done:
				return 0, zero, io.EOF
			case <-c.rdeadline.channel():
				return 0, zero, c.newError("read", os.ErrDeadlineExceeded)
			}
		}
	}
}

func (c *packetConn[T]) Write(b []byte) (int, error) {
	return c.writeTo(b, c.socket.laddr)
}

func (c *packetConn[T]) WriteTo(b []byte, addr net.Addr) (int, error) {
	sockaddr, errno := c.socket.net.sockAddr(addr)
	if errno != wasi.ESUCCESS {
		return 0, c.newError("write", errno)
	}
	return c.writeTo(b, sockaddr)
}

func (c *packetConn[T]) writeTo(b []byte, addr T) (int, error) {
	c.wmu.Lock() // serialize writes
	defer c.wmu.Unlock()

	select {
	case <-c.done:
		return 0, c.newError("write", net.ErrClosed)
	default:
	}

	if !c.socket.net.contains(addr) {
		return len(b), nil
	}

	var sbuf *sockbuf[T]
	var sev *event

	if addr == c.socket.laddr {
		sbuf = c.socket.rbuf
		sev = &c.socket.rbuf.wev
	} else {
		sock := c.socket.net.socket(netaddr[T]{c.socket.proto, addr})
		if sock == nil || sock.typ != datagram {
			return len(b), nil
		}
		sock.synchronize(func() {
			sock.allocateBuffersIfNil()
			sbuf = sock.rbuf
			sev = &sock.rbuf.wev
		})
	}

	for {
		if c.wdeadline.expired() {
			return 0, c.newError("write", os.ErrDeadlineExceeded)
		}

		n, errno := sbuf.sendmsg([]wasi.IOVec{b}, c.socket.bound)
		if errno == wasi.ESUCCESS {
			return int(n), nil
		}
		if errno == wasi.EMSGSIZE {
			return len(b), nil
		}
		if errno != wasi.EAGAIN {
			return 0, c.newError("write", errno.Syscall())
		}

		var ready bool
		sev.synchronize(func() { ready = sev.poll(c.wpoll) })

		if !ready {
			select {
			case <-c.wpoll:
			case <-c.done:
				return 0, io.EOF
			case <-c.wdeadline.channel():
				return 0, c.newError("write", os.ErrDeadlineExceeded)
			}
		}
	}
}

func (c *packetConn[T]) LocalAddr() net.Addr {
	return c.socket.laddr.netAddr(c.socket.proto)
}

func (c *packetConn[T]) RemoteAddr() net.Addr {
	return c.socket.raddr.netAddr(c.socket.proto)
}

func (c *packetConn[T]) SetDeadline(t time.Time) error {
	select {
	case <-c.done:
		return c.newError("set", net.ErrClosed)
	default:
		c.rdeadline.set(t)
		c.wdeadline.set(t)
		return nil
	}
}

func (c *packetConn[T]) SetReadDeadline(t time.Time) error {
	select {
	case <-c.done:
		return c.newError("set", net.ErrClosed)
	default:
		c.rdeadline.set(t)
		return nil
	}
}

func (c *packetConn[T]) SetWriteDeadline(t time.Time) error {
	select {
	case <-c.done:
		return c.newError("set", net.ErrClosed)
	default:
		c.wdeadline.set(t)
		return nil
	}
}

func (c *packetConn[T]) newError(op string, err error) error {
	return newConnError(op, c.LocalAddr(), c.RemoteAddr(), err)
}

func newConnError(op string, laddr, raddr net.Addr, err error) error {
	return &net.OpError{
		Op:     op,
		Net:    laddr.Network(),
		Source: laddr,
		Addr:   raddr,
		Err:    err,
	}
}

var (
	_ net.Conn       = (*packetConn[ipv4])(nil)
	_ net.PacketConn = (*packetConn[ipv4])(nil)
)

type deadline struct {
	mu sync.Mutex
	ts time.Time
	tm *time.Timer
}

func makeDeadline() deadline {
	tm := time.NewTimer(0)
	if !tm.Stop() {
		<-tm.C
	}
	return deadline{tm: tm}
}

func (d *deadline) channel() <-chan time.Time {
	return d.tm.C
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

	if !d.tm.Stop() {
		select {
		case <-d.tm.C:
		default:
		}
	}

	if !t.IsZero() {
		timeout := time.Until(t)
		if timeout < 0 {
			timeout = 0
		}
		d.tm.Reset(timeout)
	}
}

type connTunnel struct {
	refc  int32
	conn1 net.Conn
	conn2 net.Conn
	errs  chan<- wasi.Errno
}

func startConnTunnel(ctx context.Context, downstream, upstream net.Conn, rbufsize, wbufsize int, errs chan<- wasi.Errno) {
	buffer := make([]byte, rbufsize+wbufsize)
	tunnel := &connTunnel{
		refc:  2,
		conn1: downstream,
		conn2: upstream,
		errs:  errs,
	}
	go tunnel.copy(upstream, downstream, buffer[:rbufsize])
	go tunnel.copy(downstream, upstream, buffer[rbufsize:])
	go closeReadOnCancel(ctx, upstream, downstream)
}

func (c *connTunnel) unref() {
	if atomic.AddInt32(&c.refc, -1) == 0 {
		c.conn1.Close()
		c.conn2.Close()
		close(c.errs)
	}
}

func (c *connTunnel) copy(dst, src net.Conn, buf []byte) {
	defer c.unref()
	defer closeWrite(dst) //nolint:errcheck
	_, err := io.CopyBuffer(dst, src, buf)
	if err != nil {
		c.errs <- wasi.MakeErrno(err)
	}
}

type closeReader interface {
	CloseRead() error
}

type closeWriter interface {
	CloseWrite() error
}

var (
	_ closeReader = (*net.TCPConn)(nil)
	_ closeWriter = (*net.TCPConn)(nil)

	_ closeReader = (*conn[ipv4])(nil)
	_ closeWriter = (*conn[ipv4])(nil)
)

func closeRead(conn io.Closer) error {
	switch c := conn.(type) {
	case closeReader:
		return c.CloseRead()
	default:
		return c.Close()
	}
}

func closeWrite(conn io.Closer) error {
	switch c := conn.(type) {
	case closeWriter:
		return c.CloseWrite()
	default:
		return c.Close()
	}
}

func closeReadOnCancel(ctx context.Context, conn1, conn2 io.Closer) {
	<-ctx.Done()
	closeRead(conn1) //nolint:errcheck
	closeRead(conn2) //nolint:errcheck
}

func closeOnCancel(ctx context.Context, conn io.Closer) {
	<-ctx.Done()
	conn.Close() //nolint:errcheck
}

type packetTunnel[T sockaddr] struct {
	refc int32
	sock *socket[T]
	conn io.Closer
	errs chan<- wasi.Errno
}

func (s *socket[T]) startPacketTunnel(ctx context.Context, conn net.PacketConn) {
	ctx, s.cancel = context.WithCancel(ctx)

	errs := make(chan wasi.Errno, 2)
	s.errs = errs
	s.allocateBuffersIfNil()

	rbufsize := s.rbuf.size()
	wbufsize := s.wbuf.size()

	buffer := make([]byte, rbufsize+wbufsize)
	tunnel := &packetTunnel[T]{
		refc: 2,
		sock: s,
		conn: conn,
		errs: errs,
	}

	go tunnel.readFromPacketConn(conn, buffer[:rbufsize])
	go tunnel.writeToPacketConn(conn, buffer[rbufsize:])
	go closeOnCancel(ctx, conn)
}

func (s *socket[T]) startPacketTunnelTo(ctx context.Context, conn net.Conn) {
	ctx, s.cancel = context.WithCancel(ctx)

	errs := make(chan wasi.Errno, 2)
	s.errs = errs
	s.allocateBuffersIfNil()

	rbufsize := s.rbuf.size()
	wbufsize := s.wbuf.size()

	buffer := make([]byte, rbufsize+wbufsize)
	tunnel := &packetTunnel[T]{
		refc: 2,
		sock: s,
		conn: conn,
		errs: errs,
	}

	go tunnel.readFromConn(conn, buffer[:rbufsize])
	go tunnel.writeToConn(conn, buffer[rbufsize:])
	go closeOnCancel(ctx, conn)
}

func (p *packetTunnel[T]) unref() {
	if atomic.AddInt32(&p.refc, -1) == 0 {
		p.conn.Close()
		close(p.errs)
	}
}

func (p *packetTunnel[T]) readFromPacketConn(conn net.PacketConn, buf []byte) {
	network := p.sock.net
	p.readFrom(buf, func(b []byte) (int, T, error) {
		var zero T
		n, addr, err := conn.ReadFrom(b)
		if err != nil {
			return n, zero, err
		}
		peer, errno := network.sockAddr(addr)
		if errno != wasi.ESUCCESS {
			return n, zero, errno
		}
		return n, peer, nil
	})
}

func (p *packetTunnel[T]) readFromConn(conn net.Conn, buf []byte) {
	addr := p.sock.raddr
	p.readFrom(buf, func(b []byte) (int, T, error) {
		n, err := conn.Read(b)
		return n, addr, err
	})
}

func (p *packetTunnel[T]) readFrom(buf []byte, read func([]byte) (int, T, error)) {
	defer p.unref()
	defer p.sock.rbuf.close()

	for {
		size, addr, err := read(buf)
		if err != nil {
			p.errs <- wasi.MakeErrno(err)
			return
		}
		// TODO:
		// - capture metric about packets that were dropped
		// - log details about the reason why a packet was dropped
		_, errno := p.sock.rbuf.sendmsg([]wasi.IOVec{buf[:size]}, addr)
		if errno != wasi.ESUCCESS {
			continue
		}
	}
}

func (p *packetTunnel[T]) writeToPacketConn(conn net.PacketConn, buf []byte) {
	proto := p.sock.proto
	p.writeTo(buf, func(b []byte, a T) (int, error) { return conn.WriteTo(b, a.netAddr(proto)) })
}

func (p *packetTunnel[T]) writeToConn(conn net.Conn, buf []byte) {
	p.writeTo(buf, func(b []byte, _ T) (int, error) { return conn.Write(b) })
}

func (p *packetTunnel[T]) writeTo(buf []byte, write func([]byte, T) (int, error)) {
	defer p.unref()
	defer p.sock.wbuf.close()

	signal := make(chan struct{}, 1)
	for {
		size, _, addr, errno := p.sock.wbuf.recvmsg([]wasi.IOVec{buf}, 0)
		switch errno {
		case wasi.ESUCCESS:
			if size == 0 {
				return
			}
			_, err := write(buf[:size], addr)
			if err != nil {
				p.errs <- wasi.MakeErrno(err)
				return
			}
		case wasi.EAGAIN:
			var ready bool
			p.sock.wev.synchronize(func() { ready = p.sock.wev.poll(signal) })
			if !ready {
				<-signal
			}
		default:
			// TODO:
			// - log details about the reason why we abort
			p.errs <- errno
			return
		}
	}
}

type listenTunnel[T sockaddr] struct {
	listener net.Listener
	socket   *socket[T]
	rbufsize int32
	wbufsize int32
	errs     chan<- wasi.Errno
}

func (s *socket[T]) startListenTunnel(ctx context.Context, l net.Listener) {
	ctx, s.cancel = context.WithCancel(ctx)

	errs := make(chan wasi.Errno, 1)
	s.errs = errs

	tunnel := listenTunnel[T]{
		listener: l,
		socket:   s,
		rbufsize: s.rbufsize,
		wbufsize: s.wbufsize,
		errs:     errs,
	}

	go tunnel.acceptConnections()
	go closeOnCancel(ctx, l)
}

func (l listenTunnel[T]) acceptConnections() {
	defer l.listener.Close()
	defer l.socket.close()
	defer close(l.errs)

	for {
		downstream, err := l.listener.Accept()

		if err != nil {
			if isTemporary(err) {
				continue
			} else {
				l.errs <- wasi.MakeErrno(err)
				return
			}
		}

		socket := l.socket.newSocket()
		socket.flags = socket.flags.with(sockConn)

		ctx, cancel := context.WithCancel(context.Background())
		socket.cancel = cancel

		errs := make(chan wasi.Errno, 2)
		socket.errs = errs

		raddr, _ := l.socket.net.sockAddr(downstream.RemoteAddr())

		errno := l.socket.connect(nil, socket, l.socket.laddr, raddr)
		if errno != wasi.ESUCCESS {
			downstream.Close()
			continue
		}

		upstream := newHostConn(socket)
		rbufsize := int(l.rbufsize)
		wbufsize := int(l.wbufsize)
		startConnTunnel(ctx, downstream, upstream, rbufsize, wbufsize, errs)
	}
}

func isTemporary(err error) bool {
	e, _ := err.(interface{ Temporary() bool })
	return e != nil && e.Temporary()
}
