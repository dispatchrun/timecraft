//nolint:unused
package sandbox

import (
	"context"
	"io"
	"net"
	"sync"
	"sync/atomic"

	"github.com/stealthrocket/wasi-go"
)

type ringbuf[T any] struct {
	buf []T
	off int
	end int
}

func (rb *ringbuf[T]) len() int {
	return rb.end - rb.off
}

func (rb *ringbuf[T]) cap() int {
	return len(rb.buf)
}

func (rb *ringbuf[T]) avail() int {
	return rb.off + (len(rb.buf) - rb.end)
}

func (rb *ringbuf[T]) index(i int) *T {
	return &rb.buf[rb.off+i]
}

func (rb *ringbuf[T]) discard(n int) {
	rb.off += n
}

func (rb *ringbuf[T]) reset() {
	rb.off = 0
	rb.end = 0
}

func (rb *ringbuf[T]) read(values []T) int {
	n := copy(values, rb.buf[rb.off:rb.end])
	if rb.off += n; rb.off == rb.end {
		rb.off = 0
		rb.end = 0
	}
	return n
}

func (rb *ringbuf[T]) write(values []T) int {
	if (len(rb.buf) - rb.end) < len(values) {
		if rb.off > 0 {
			rb.end = copy(rb.buf, rb.buf[rb.off:rb.end])
			rb.off = 0
		}
	}
	n := copy(rb.buf[rb.end:], values)
	rb.end += n
	return n
}

type packet[T sockaddr] struct {
	addr T
	size int
}

type sockbuf[T sockaddr] struct {
	mu  sync.Mutex
	src ringbuf[packet[T]]
	buf ringbuf[byte]
}

func (sb *sockbuf[T]) len() int {
	sb.mu.Lock()
	n := sb.buf.len()
	sb.mu.Unlock()
	return n
}

func (sb *sockbuf[T]) reset() {
	sb.mu.Lock()
	sb.src.reset()
	sb.buf.reset()
	sb.mu.Unlock()
}

func (sb *sockbuf[T]) recv(b []byte) (n int, addr T, errno wasi.Errno) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.src.len() == 0 {
		return -1, addr, wasi.EAGAIN
	}

	n = len(b)
	p := sb.src.index(0)
	if p.size < n {
		n = p.size
	}
	sb.buf.read(b[:n])
	addr = p.addr
	p.size -= n
	if p.size == 0 {
		var discard [1]packet[T]
		sb.src.read(discard[:])
	}
	return n, addr, wasi.ESUCCESS
}

func (sb *sockbuf[T]) recvmsg(b []byte) (n int, trunc bool, addr T, errno wasi.Errno) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.src.len() == 0 {
		return -1, false, addr, wasi.EAGAIN
	}

	n = len(b)
	p := sb.src.index(0)
	switch {
	case p.size < n:
		n = p.size
	case p.size > n:
		trunc = true
	}
	sb.buf.read(b[:n])
	sb.buf.discard(p.size - n)
	addr = p.addr

	var discard [1]packet[T]
	sb.src.read(discard[:])
	return n, addr, wasi.ESUCCESS
}

func (sb *sockbuf[T]) send(b []byte, addr T) (int, wasi.Errno) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.buf.avail() == 0 {
		return -1, wasi.EAGAIN
	}
	size := sb.buf.write(b)
	sb.src.write([]packet[T]{
		addr: addr,
		size: size,
	})
	return size, wasi.ESUCCESS
}

func (sb *sockbuf[T]) sendmsg(b []byte, addr T) (int, wasi.Errno) {
	sb.mu.Lock()
	defer sb.mu.Unlock()

	if sb.buf.cap() < len(b) {
		return -1, wasi.EMSGSIZE
	}
	if sb.buf.avail() < len(b) {
		return -1, wasi.EAGAIN
	}
	size := sb.buf.write(b)
	sb.src.write([]packet[T]{
		addr: addr,
		size: size,
	})
	return size, wasi.ESUCCESS
}

type socktype wasi.SocketType

const (
	datagram = socktype(wasi.DatagramSocket)
	stream   = socktype(wasi.StreamSocket)
)

func (st socktype) supports(proto protocol) bool {
	switch st {
	case datagram:
		return proto == ip || proto == udp
	case stream:
		return proto == ip || proto == tcp
	default:
		return false
	}
}

type socket[T sockaddr] struct {
	defaultFile
	net   network[T]
	typ   socktype
	proto protocol
	raddr T
	laddr T
	bound T
	// For listening sockets, this event and channel are used to pass
	// connections between the two ends of the network.
	mutex  sync.Mutex
	accept chan *socket[T]
	// Connected sockets have a bidirectional pipe that links between the two
	// ends of the socket pair.
	send *pipe // socket send end (guest side)
	recv *pipe // socket recv end (guest side)
	// This field is set to true if the socket was created by the host, which
	// allows listeners to detect if they are accepting connections from the
	// guest, and construct the right connection type.
	host bool
	// A boolean used to indicate that this is a connected socket.
	conn bool
	// For connected sockets, this channel is used to asynchronously receive
	// notification that a connection has been established.
	errs <-chan wasi.Errno
	// This cancellation function controls the lifetime of connections dialed
	// from the socket.
	cancel context.CancelFunc
	// Sizes of the receive and send buffers; must be configured prior to
	// connecting or accepting connections or it is ignored.
	recvBufferSize int32
	sendBufferSize int32
}

const (
	defaultSocketBufferSize = 16384
	minSocketBufferSize     = 1024
	maxSocketBufferSize     = 65536
)

func newSocket[T sockaddr](net network[T], lock *sync.Mutex, typ socktype, proto protocol) *socket[T] {
	return &socket[T]{
		net:   net,
		typ:   typ,
		proto: proto,
		send:  newPipe(lock),
		recv:  newPipe(lock),
		// socket options
		recvBufferSize: defaultSocketBufferSize,
		sendBufferSize: defaultSocketBufferSize,
	}
}

func (s *socket[T]) close() {
	_ = s.net.unlink(s)
	s.recv.close()
	s.send.close()
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

// TODO: remove host boolean
func (s *socket[T]) connect(laddr, raddr T, host bool) (*socket[T], wasi.Errno) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.accept == nil {
		return nil, wasi.ECONNREFUSED
	}

	// The lock is the same on both the send and receive ends.
	lock := s.send.ev.lock
	sock := newSocket[T](s.net, lock, s.typ, s.proto)
	sock.host = host
	sock.conn = true
	sock.laddr = laddr
	sock.raddr = raddr
	sock.recvBufferSize = s.recvBufferSize
	sock.sendBufferSize = s.sendBufferSize

	select {
	case s.accept <- sock:
		s.recv.ev.trigger()
		return sock, wasi.ESUCCESS
	default:
		return nil, wasi.ECONNREFUSED
	}
}

func (s *socket[T]) FDPoll(ev wasi.EventType, ch chan<- struct{}) bool {
	switch ev {
	case wasi.FDReadEvent:
		return s.recv.ev.poll(ch)
	case wasi.FDWriteEvent:
		return s.send.ev.poll(ch)
	default:
		return false
	}
}

func (s *socket[T]) SockListen(ctx context.Context, backlog int) wasi.Errno {
	if s.conn {
		return wasi.EINVAL
	}
	if backlog <= 0 || backlog > 128 {
		backlog = 128
	}
	if s.typ != stream {
		return wasi.ENOTSUP
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.accept == nil {
		var zero T
		if s.laddr == zero {
			if errno := s.net.bind(zero, s); errno != wasi.ESUCCESS {
				return errno
			}
		}
		s.accept = make(chan *socket[T], backlog)
	}

	return wasi.ESUCCESS
}

func (s *socket[T]) SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno) {
	if s.typ == datagram {
		return nil, wasi.ENOTSUP
	}

	s.mutex.Lock()
	accept := s.accept
	s.mutex.Unlock()

	if accept == nil {
		return nil, wasi.EINVAL
	}

	var sock *socket[T]
	s.recv.ev.clear()

	if s.recv.flags.Has(wasi.NonBlock) {
		select {
		case sock = <-accept:
		case <-s.recv.done:
			return nil, wasi.EBADF
		default:
			return nil, wasi.EAGAIN
		}
	} else {
		select {
		case sock = <-accept:
		case <-s.recv.done:
			return nil, wasi.EBADF
		case <-ctx.Done():
			return nil, wasi.MakeErrno(ctx.Err())
		}
	}

	if sock == nil {
		return nil, wasi.EINVAL
	}

	if len(accept) > 0 {
		s.recv.ev.trigger()
	}
	_ = sock.FDStatSetFlags(ctx, flags)
	return sock, wasi.ESUCCESS
}

func (s *socket[T]) SockBind(ctx context.Context, bind wasi.SocketAddress) wasi.Errno {
	addr, errno := s.net.sockaddr(bind)
	if errno != wasi.ESUCCESS {
		return errno
	}
	var zero T
	if s.laddr != zero {
		return wasi.EINVAL
	}
	return s.net.bind(addr, s)
}

func (s *socket[T]) SockConnect(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	raddr, errno := s.net.sockaddr(addr)
	if errno != wasi.ESUCCESS {
		return errno
	}

	// Stream sockets cannot be connected if they are listening, nor reconnected
	// if a connection has already been initiated.
	if s.typ == stream {
		if s.accept != nil {
			return wasi.EISCONN
		}
		if s.conn {
			return wasi.EALREADY
		}
	}

	// Automtically assign a local address to the socket if it was not already
	// bound ahead of time.
	var zero T
	if s.laddr == zero {
		if errno := s.net.link(s); errno != wasi.ESUCCESS {
			return errno
		}
	}
	s.raddr = raddr

	// Datagram sockets can be reconnected, so we must cancel any previous
	// connetion.
	if s.cancel != nil {
		s.send.close()
		s.recv.close()
		s.cancel()
		s.cancel = nil
		for range s.errs {
		}
		s.errs = nil
		s.send = newPipe(s.send.ev.lock)
		s.recv = newPipe(s.recv.ev.lock)
	}

	ctx, s.cancel = context.WithCancel(ctx)
	// At most three errors are produced to this channel, one when the dial
	// function failed or the connection is blocking, and up to two if both
	// the read and write pipes error.
	errs := make(chan wasi.Errno, 3)
	s.errs = errs
	s.conn = true

	if s.typ == datagram {
		var conn net.Conn
		var errno wasi.Errno
		// We assume that establishing connections for datagram connections
		// do not block, so we can do it synchronously.
		if s.net.contains(s.raddr) {
			conn = newPacketConn(s)
		} else {
			conn, errno = s.net.dial(ctx, s.proto, s.laddr, s.raddr)
		}
		if errno != wasi.ESUCCESS {
			return errno
		}
		spawnSocketConn(ctx, conn, s.send, s.recv, s.sendBufferSize, s.recvBufferSize, errs)
		s.send.ev.trigger()
		return wasi.ESUCCESS
	}

	blocking := !s.send.flags.Has(wasi.NonBlock)
	network := s.net
	proto := s.proto
	laddr := s.laddr
	send := s.send
	recv := s.recv
	recvBufferSize := s.recvBufferSize
	sendBufferSize := s.sendBufferSize

	go func() {
		var conn net.Conn
		var errno wasi.Errno

		if network.contains(raddr) {
			sock := network.socket(netaddr[T]{proto, raddr})
			if sock == nil {
				errno = wasi.ECONNREFUSED
			} else {
				sock, errno = sock.connect(raddr, laddr, false)
				if errno == wasi.ESUCCESS {
					conn = newHostConn(sock)
				}
			}
		} else {
			conn, errno = network.dial(ctx, proto, laddr, raddr)
		}

		if errno != wasi.ESUCCESS || blocking {
			errs <- errno
		}

		if errno != wasi.ESUCCESS {
			send.close()
			recv.close()
			close(errs)
		} else {
			send.ev.trigger()
			spawnSocketConn(ctx, conn, send, recv, sendBufferSize, recvBufferSize, errs)
		}
	}()

	if !blocking {
		return wasi.EINPROGRESS
	}

	select {
	case errno := <-errs:
		return errno
	case <-ctx.Done():
		s.cancel()
		return wasi.MakeErrno(ctx.Err())
	}
}

type connRef struct {
	refc int32
	conn net.Conn
	errs chan<- wasi.Errno
}

func spawnSocketConn(ctx context.Context, conn net.Conn, send, recv *pipe, sendBufferSize, recvBufferSize int32, errs chan<- wasi.Errno) {
	// TODO: pool the buffers?
	sockBuf := make([]byte, recvBufferSize+sendBufferSize)
	connRef := &connRef{refc: 2, conn: conn, errs: errs}
	go connRef.copy(send, conn, outputReadCloser{send}, sockBuf[:recvBufferSize])
	go connRef.copy(recv, inputWriteCloser{recv}, conn, sockBuf[recvBufferSize:])
	go closeOnCancel(ctx, conn, send, recv)
}

func (c *connRef) unref() {
	if atomic.AddInt32(&c.refc, -1) == 0 {
		c.conn.Close()
		close(c.errs)
	}
}

func (c *connRef) copy(p *pipe, w io.Writer, r io.Reader, b []byte) {
	_, err := io.CopyBuffer(w, r, b)
	if err != nil {
		c.errs <- wasi.MakeErrno(err)
	}
	p.close()
	c.unref()
}

func closeOnCancel(ctx context.Context, conn net.Conn, send, recv *pipe) {
	<-ctx.Done()
	conn.Close()
	send.close()
	recv.close()
}

func (s *socket[T]) SockRecv(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	if errno := s.getErrno(); errno != wasi.ESUCCESS {
		return 0, 0, errno
	}
	// TODO:
	// - RecvPeek
	// - RecvWaitAll
	// - RecvDataTruncated
	size, errno := s.recv.ch.read(ctx, iovs, s.recv.flags, &s.recv.ev, nil, s.recv.done)
	return size, 0, errno
}

func (s *socket[T]) SockRecvFrom(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	if s.accept != nil {
		return ^wasi.Size(0), 0, nil, wasi.ENOTSUP
	}
	if s.conn {
		return ^wasi.Size(0), 0, nil, wasi.EISCONN
	}
	size, errno := s.recv.ch.read(ctx, iovs, s.recv.flags, &s.recv.ev, nil, s.recv.done)
	return size, 0, nil, errno
}

func (s *socket[T]) SockSend(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	if errno := s.getErrno(); errno != wasi.ESUCCESS {
		return 0, errno
	}
	return s.send.ch.write(ctx, iovs, s.send.flags, &s.send.ev, nil, s.send.done)
}

func (s *socket[T]) SockSendTo(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	if s.accept != nil {
		return ^wasi.Size(0), wasi.ENOTSUP
	}
	if s.conn {
		return ^wasi.Size(0), wasi.EISCONN
	}

	var zero T
	if s.laddr == zero {
		if errno := s.net.bind(zero, s); errno != wasi.ESUCCESS {
			return ^wasi.Size(0), errno
		}
	}

	if errno := s.getErrno(); errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}

	toAddr, errno := s.net.sockaddr(addr)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}

	msgLen := iovecLen(iovs)
	toSock := s.net.socket(netaddr[T]{s.proto, toAddr})
	if toSock == nil {
		return wasi.Size(msgLen), wasi.ESUCCESS
	}

	// This is far from perfect because the program may change the send buffer
	// size after the socket was connected which may then result in sending a
	// truncated message instead of returning EMSGSIZE (because we don't resize
	// socket buffers after connecting).
	if msgLen > int(s.sendBufferSize) {
		return 0, wasi.EMSGSIZE
	}

	send := toSock.send
	_, errno = send.ch.write(ctx, iovs, send.flags, &send.ev, nil, send.done)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	return wasi.Size(msgLen), wasi.ESUCCESS
}

func iovecLen(iovs []wasi.IOVec) (size int) {
	for _, iov := range iovs {
		size += len(iov)
	}
	return size
}

func (s *socket[T]) SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return s.laddr.sockAddr(), wasi.ESUCCESS
}

func (s *socket[T]) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return s.raddr.sockAddr(), wasi.ESUCCESS
}

func (s *socket[T]) SockGetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	switch level {
	case wasi.SocketLevel:
		return s.getSocketLevelOption(option)
	default:
		return nil, wasi.EINVAL
	}
}

func (s *socket[T]) getSocketLevelOption(option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	switch option {
	case wasi.ReuseAddress:
	case wasi.QuerySocketType:
		return wasi.IntValue(s.typ), wasi.ESUCCESS
	case wasi.QuerySocketError:
		return wasi.IntValue(s.getErrno()), wasi.ESUCCESS
	case wasi.DontRoute:
	case wasi.Broadcast:
	case wasi.SendBufferSize:
		return wasi.IntValue(s.sendBufferSize), wasi.ESUCCESS
	case wasi.RecvBufferSize:
		return wasi.IntValue(s.recvBufferSize), wasi.ESUCCESS
	case wasi.KeepAlive:
	case wasi.OOBInline:
	case wasi.Linger:
	case wasi.RecvLowWatermark:
	case wasi.RecvTimeout:
	case wasi.SendTimeout:
	case wasi.QueryAcceptConnections:
	case wasi.BindToDevice:
	}
	return nil, wasi.EINVAL
}

func (s *socket[T]) SockSetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	switch level {
	case wasi.SocketLevel:
		return s.setSocketLevelOption(option, value)
	default:
		return wasi.EINVAL
	}
}

func (s *socket[T]) setSocketLevelOption(option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	switch option {
	case wasi.ReuseAddress:
	case wasi.QuerySocketType:
	case wasi.QuerySocketError:
	case wasi.DontRoute:
	case wasi.Broadcast:
	case wasi.SendBufferSize:
		return setIntValueLimit(&s.sendBufferSize, value, minSocketBufferSize, maxSocketBufferSize)
	case wasi.RecvBufferSize:
		return setIntValueLimit(&s.recvBufferSize, value, minSocketBufferSize, maxSocketBufferSize)
	case wasi.KeepAlive:
	case wasi.OOBInline:
	case wasi.Linger:
	case wasi.RecvLowWatermark:
	case wasi.RecvTimeout:
	case wasi.SendTimeout:
	case wasi.QueryAcceptConnections:
	case wasi.BindToDevice:
	}
	return wasi.EINVAL
}

func setIntValueLimit(option *int32, value wasi.SocketOptionValue, minval, maxval int32) wasi.Errno {
	switch v := value.(type) {
	case wasi.IntValue:
		if v32 := int32(v); v32 >= 0 {
			if v32 < minval {
				v32 = minval
			}
			if v32 > maxval {
				v32 = maxval
			}
			*option = v32
			return wasi.ESUCCESS
		}
	}
	return wasi.EINVAL
}

func (s *socket[T]) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	if !s.conn {
		return wasi.ENOTCONN
	}

	if flags.Has(wasi.ShutdownRD) {
		if s.recv.ev.state() == aborted {
			return wasi.ENOTCONN
		}
		s.recv.close()
	}

	if flags.Has(wasi.ShutdownWR) {
		if s.send.ev.state() == aborted {
			return wasi.ENOTCONN
		}
		s.send.close()
	}

	return wasi.ESUCCESS
}

func (s *socket[T]) FDClose(ctx context.Context) wasi.Errno {
	s.close()

	if s.errs != nil {
		// Drain the errors channel to make sure that all sync operations on the
		// socket have completed.
		for range s.errs {
		}
	}

	s.mutex.Lock()
	accept := s.accept
	s.accept = nil
	s.mutex.Unlock()

	if accept != nil {
		close(accept)
		for sock := range accept {
			_ = sock.FDClose(ctx)
		}
	}
	return wasi.ESUCCESS
}

func (s *socket[T]) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	_ = s.send.FDStatSetFlags(ctx, flags)
	_ = s.recv.FDStatSetFlags(ctx, flags)
	return wasi.ESUCCESS
}

func (s *socket[T]) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	size, _, errno := s.SockRecv(ctx, iovs, 0)
	return size, errno
}

func (s *socket[T]) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.SockSend(ctx, iovs, 0)
}

func (s *socket[T]) getErrno() wasi.Errno {
	select {
	case errno := <-s.errs:
		return errno
	default:
		return wasi.ESUCCESS
	}
}
