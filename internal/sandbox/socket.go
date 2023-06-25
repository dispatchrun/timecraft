//nolint:unused
package sandbox

import (
	"context"
	"sync"
	"time"

	"github.com/stealthrocket/wasi-go"
)

func sumIOVecLen(iovs []wasi.IOVec) (n int) {
	for _, iov := range iovs {
		n += len(iov)
	}
	return n
}

type sockflags uint32

const (
	sockClosed sockflags = 1 << iota
	sockConn
	sockHost
	sockListen
	sockNonBlock
)

func (f sockflags) has(flags sockflags) bool {
	return (f & flags) != 0
}

func (f sockflags) with(flags sockflags) sockflags {
	return f | flags
}

func (f sockflags) without(flags sockflags) sockflags {
	return f & ^flags
}

func (f sockflags) withFDFlags(flags wasi.FDFlags) sockflags {
	if flags.Has(wasi.NonBlock) {
		return f.with(sockNonBlock)
	} else {
		return f.without(sockNonBlock)
	}
}

type packet[T sockaddr] struct {
	addr T
	size int
}

type sockbuf[T sockaddr] struct {
	mu  sync.Mutex
	src ringbuf[packet[T]]
	buf ringbuf[byte]
	rev event
	wev event
}

func newSocketBuffer[T sockaddr](lock *sync.Mutex, n int) *sockbuf[T] {
	sb := &sockbuf[T]{
		buf: makeRingBuffer[byte](n),
		rev: makeEvent(lock),
		wev: makeEvent(lock),
	}
	if n > 0 {
		sb.wev.trigger()
	}
	return sb
}

func (sb *sockbuf[T]) close() {
	sb.rev.abort()
	sb.wev.abort()
}

func (sb *sockbuf[T]) lock() {
	sb.mu.Lock()
}

func (sb *sockbuf[T]) unlock() {
	sb.rev.update(sb.buf.len() != 0)
	sb.wev.update(sb.buf.avail() != 0)
	sb.mu.Unlock()
}

func (sb *sockbuf[T]) size() int {
	sb.mu.Lock()
	size := sb.buf.cap()
	sb.mu.Unlock()
	return size
}

func (sb *sockbuf[T]) recv(iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, T, wasi.Errno) {
	sb.lock()
	defer sb.unlock()

	var addr T
	if sb.src.len() == 0 {
		if sb.rev.state() == aborted {
			return 0, 0, addr, wasi.ESUCCESS
		}
		return ^wasi.Size(0), 0, addr, wasi.EAGAIN
	}

	size := sumIOVecLen(iovs)
	packet := sb.src.index(0)
	addr = packet.addr
	if packet.size < size {
		size = packet.size
	}

	remain := size
	for _, iov := range iovs {
		if remain < len(iov) {
			iov = iov[:remain]
		}
		n := sb.buf.peek(iov, size-remain)
		if remain -= n; remain == 0 {
			break
		}
	}

	if !flags.Has(wasi.RecvPeek) {
		sb.buf.discard(size)
		packet.size -= size
		if packet.size == 0 {
			sb.src.discard(1)
		}
	}
	return wasi.Size(size), 0, addr, wasi.ESUCCESS
}

func (sb *sockbuf[T]) recvmsg(iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, T, wasi.Errno) {
	sb.lock()
	defer sb.unlock()

	var addr T
	if sb.src.len() == 0 {
		if sb.rev.state() == aborted {
			return 0, 0, addr, wasi.ESUCCESS
		}
		return ^wasi.Size(0), 0, addr, wasi.EAGAIN
	}

	size := sumIOVecLen(iovs)
	packet := sb.src.index(0)
	addr = packet.addr
	var roflags wasi.ROFlags
	switch {
	case packet.size < size:
		size = packet.size
	case packet.size > size:
		roflags |= wasi.RecvDataTruncated
	}

	remain := size
	for _, iov := range iovs {
		if remain < len(iov) {
			iov = iov[:remain]
		}
		n := sb.buf.peek(iov, size-remain)
		if remain -= n; remain == 0 {
			break
		}
	}

	if !flags.Has(wasi.RecvPeek) {
		sb.buf.discard(packet.size)
		sb.src.discard(1)
	}
	return wasi.Size(size), roflags, addr, wasi.ESUCCESS
}

func (sb *sockbuf[T]) send(iovs []wasi.IOVec, addr T) (wasi.Size, wasi.Errno) {
	sb.lock()
	defer sb.unlock()

	if sb.buf.avail() == 0 {
		var errno wasi.Errno
		if sb.wev.state() == aborted {
			errno = wasi.ECONNRESET
		} else {
			errno = wasi.EAGAIN
		}
		return ^wasi.Size(0), errno
	}

	var size int
	for _, iov := range iovs {
		n := sb.buf.write(iov)
		size += n
		if n < len(iov) {
			break
		}
	}

	sb.src.append(packet[T]{
		addr: addr,
		size: size,
	})
	return wasi.Size(size), wasi.ESUCCESS
}

func (sb *sockbuf[T]) sendmsg(iovs []wasi.IOVec, addr T) (wasi.Size, wasi.Errno) {
	sb.lock()
	defer sb.unlock()

	if sb.wev.state() == aborted {
		return ^wasi.Size(0), wasi.ENOTCONN
	}
	if sb.buf.avail() == 0 {
		return ^wasi.Size(0), wasi.EAGAIN
	}

	size := sumIOVecLen(iovs)
	if sb.buf.cap() < int(size) {
		return ^wasi.Size(0), wasi.EMSGSIZE
	}
	if sb.buf.avail() < int(size) {
		return ^wasi.Size(0), wasi.EAGAIN
	}

	for _, iov := range iovs {
		sb.buf.write(iov)
	}

	sb.src.append(packet[T]{
		addr: addr,
		size: size,
	})
	return wasi.Size(size), wasi.ESUCCESS
}

type socktype wasi.SocketType

const (
	datagram = socktype(wasi.DatagramSocket)
	stream   = socktype(wasi.StreamSocket)
)

func (st socktype) String() string {
	switch st {
	case datagram:
		return "datagram"
	case stream:
		return "stream"
	default:
		return "unknown"
	}
}

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
	mutex sync.Mutex
	flags sockflags
	// Events indicating readiness for read or write operations on the socket.
	rev *event
	wev *event
	// For listening sockets, this event and channel are used to pass
	// connections between the two ends of the network.
	accept chan *socket[T]
	// Connected sockets have a bidirectional pipe that links between the two
	// ends of the socket pair.
	rbuf *sockbuf[T] // socket recv end (guest side)
	wbuf *sockbuf[T] // socket send end (guest side)
	// Functions used to send and receive message on the socket, implement the
	// difference in behavior between stream and datagram sockets.
	sendmsg func(*sockbuf[T], []wasi.IOVec, T) (wasi.Size, wasi.Errno)
	recvmsg func(*sockbuf[T], []wasi.IOVec, wasi.RIFlags) (wasi.Size, wasi.ROFlags, T, wasi.Errno)
	// For connected sockets, this channel is used to asynchronously receive
	// notification that a connection has been established.
	errs <-chan wasi.Errno
	// This cancellation function controls the lifetime of connections dialed
	// from the socket.
	cancel context.CancelFunc
	// Sizes of the receive and send buffers; must be configured prior to
	// connecting or accepting connections or it is ignored.
	rbufsize int32
	wbufsize int32
	// Timeouts applied when the socket is in blocking mode; configured with the
	// RecvTimeout and SendTimeout socket options. Zero means no timeout.
	rtimeout time.Duration
	wtimeout time.Duration
	// In blocking mode, this channel is used to poll for read/write operations
	// and block until the socket becomes ready. The channel is shared with the
	// System instance that created the socket since a blocking socket method
	// will prevent any concurrent call to PollOneOff.
	poll chan struct{}
}

const (
	defaultSocketBufferSize = 16384
	minSocketBufferSize     = 1024
	maxSocketBufferSize     = 65536
)

func newSocket[T sockaddr](net network[T], typ socktype, proto protocol, lock *sync.Mutex, poll chan struct{}) *socket[T] {
	events := [...]event{
		0: makeEvent(lock),
		1: makeEvent(lock),
	}
	sock := &socket[T]{
		net:      net,
		typ:      typ,
		proto:    proto,
		rev:      &events[0],
		wev:      &events[1],
		rbufsize: defaultSocketBufferSize,
		wbufsize: defaultSocketBufferSize,
		poll:     poll,
	}
	if typ == datagram {
		sock.sendmsg = (*sockbuf[T]).sendmsg
		sock.recvmsg = (*sockbuf[T]).recvmsg
	} else {
		sock.sendmsg = (*sockbuf[T]).send
		sock.recvmsg = (*sockbuf[T]).recv
	}
	return sock
}

func (s *socket[T]) close() {
	s.mutex.Lock()
	rbuf := s.rbuf
	wbuf := s.wbuf
	errs := s.errs
	accept := s.accept
	cancel := s.cancel
	closed := (s.flags & sockClosed) != 0
	s.flags = s.flags.with(sockClosed)
	s.mutex.Unlock()

	if closed {
		return
	}

	_ = s.net.unlink(s)
	s.rev.abort()
	s.wev.abort()

	if rbuf != nil {
		rbuf.close()
	}
	if wbuf != nil {
		wbuf.close()
	}
	if cancel != nil {
		cancel()
	}

	if errs != nil {
		for range errs {
		}
	}

	if accept != nil {
		close(accept)
		for sock := range accept {
			sock.close()
		}
	}
}

func (s *socket[T]) connect(peer *socket[T], laddr, raddr T) (*socket[T], wasi.Errno) {
	lock := s.rev.lock
	sock := newSocket[T](s.net, s.typ, s.proto, lock, s.poll)
	sock.flags = sock.flags.with(sockConn)
	sock.laddr = laddr
	sock.raddr = raddr

	if peer == nil {
		sock.flags = sock.flags.with(sockHost)
		sock.rbuf = newSocketBuffer[T](lock, int(sock.rbufsize))
		sock.wbuf = newSocketBuffer[T](lock, int(sock.wbufsize))
	} else {
		// Sockets paired on the same network share the same receive and send
		// buffers, but swapped so data sent by the peer is read by the socket
		// vice versa.
		sock.rbuf = peer.wbuf
		sock.wbuf = peer.rbuf
	}

	sock.rev = &sock.rbuf.rev
	sock.wev = &sock.wbuf.wev

	if sock.typ == datagram {
		// When connecting a datagram socket, the new socket must be bound to
		// the network in order to have an address to receive packets on.
		if errno := sock.net.link(sock); errno != wasi.ESUCCESS {
			return nil, errno
		}
		sock.allocateBuffersIfNil()
		return sock, wasi.ESUCCESS
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.flags.has(sockClosed) || s.accept == nil {
		return nil, wasi.ECONNREFUSED
	}

	select {
	case s.accept <- sock:
		s.rev.trigger()
		return sock, wasi.ESUCCESS
	default:
		return nil, wasi.ECONNREFUSED
	}
}

func (s *socket[T]) synchronize(f func()) {
	synchronize(&s.mutex, f)
}

func wait(ctx context.Context, mu *sync.Mutex, ev *event, timeout time.Duration, poll chan struct{}) wasi.Errno {
	if poll == nil {
		return wasi.ENOTSUP
	}

	var ready bool
	ev.synchronize(func() { ready = ev.poll(poll) })

	if !ready {
		if mu != nil {
			mu.Unlock()
			defer mu.Lock()
		}

		var deadline <-chan time.Time
		if timeout > 0 {
			tm := time.NewTimer(timeout)
			deadline = tm.C
			defer tm.Stop()
		}

		select {
		case <-poll:
		case <-deadline:
			return wasi.EAGAIN
		case <-ctx.Done():
			return wasi.MakeErrno(ctx.Err())
		}
	}
	return wasi.ESUCCESS
}

func (s *socket[T]) FDPoll(ev wasi.EventType, ch chan<- struct{}) bool {
	switch ev {
	case wasi.FDReadEvent:
		return s.rev.poll(ch)
	case wasi.FDWriteEvent:
		return s.wev.poll(ch)
	default:
		return false
	}
}

func (s *socket[T]) SockListen(ctx context.Context, backlog int) wasi.Errno {
	if s.typ != stream {
		return wasi.ENOTSUP
	}
	if backlog <= 0 || backlog > 128 {
		backlog = 128
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.flags.has(sockClosed | sockConn) {
		return wasi.EINVAL
	}

	if !s.flags.has(sockListen) {
		if errno := s.bindToAny(ctx); errno != wasi.ESUCCESS {
			return errno
		}
		s.flags = s.flags.with(sockListen)
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
	closed := s.flags.has(sockClosed)
	blocking := !s.flags.has(sockNonBlock)
	s.mutex.Unlock()

	if accept == nil || closed {
		return nil, wasi.EINVAL
	}

	var sock *socket[T]
	s.rev.clear()
	// Blocking sockets can use the accept channel as a wait point since it
	// will block the calling goroutine but would also allow for asynchronous
	// cancellation if the socket is closed by concurrent operation.
	//
	// The non-blocking status should never be updated concurrently since
	// FDStatSetFlags would only be called from the parent System on the same
	// goroutine, therefore we don't have to concern ourselves with allowing
	// the socket to be unblocked concurrently through any other means.
	if blocking {
		select {
		case sock = <-accept:
		case <-ctx.Done():
			return nil, wasi.MakeErrno(ctx.Err())
		}
	} else {
		select {
		case sock = <-accept:
		default:
			return nil, wasi.EAGAIN
		}
	}

	if sock == nil {
		// This condition may occur when the socket is closed concurrently,
		// which closes the accept channel and results in receiving nil
		// values without blocking.
		return nil, wasi.EINVAL
	}

	if len(accept) > 0 {
		s.rev.trigger()
	}
	sock.flags = sock.flags.withFDFlags(flags)
	return sock, wasi.ESUCCESS
}

func (s *socket[T]) SockBind(ctx context.Context, bind wasi.SocketAddress) wasi.Errno {
	addr, errno := s.net.sockaddr(bind)
	if errno != wasi.ESUCCESS {
		return errno
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	var zero T
	if s.flags.has(sockClosed) || s.laddr != zero {
		return wasi.EINVAL
	}
	return s.bind(ctx, func() wasi.Errno { return s.net.bind(addr, s) })
}

func (s *socket[T]) bindToAny(ctx context.Context) wasi.Errno {
	var zero T
	return s.bind(ctx, func() wasi.Errno { return s.net.bind(zero, s) })
}

func (s *socket[T]) bindToNetwork(ctx context.Context) wasi.Errno {
	return s.bind(ctx, func() wasi.Errno { return s.net.link(s) })
}

func (s *socket[T]) bind(ctx context.Context, bind func() wasi.Errno) wasi.Errno {
	var zero T
	if s.laddr != zero {
		return wasi.ESUCCESS
	}
	if s.flags.has(sockConn) {
		return wasi.ESUCCESS
	}
	if s.typ == datagram {
		switch errno := s.openPacketTunnel(ctx); errno {
		case wasi.ESUCCESS:
		case wasi.ENOTSUP:
		default:
			return errno
		}
	}
	return bind()
}

func (s *socket[T]) openPacketTunnel(ctx context.Context) wasi.Errno {
	if s.errs != nil {
		return wasi.ESUCCESS
	}
	conn, errno := s.net.listenPacket(ctx, s.proto)
	if errno != wasi.ESUCCESS {
		return errno
	}
	ctx, s.cancel = context.WithCancel(ctx)
	errs := make(chan wasi.Errno, 2)
	s.errs = errs

	rbufsize := s.rbuf.size()
	wbufsize := s.wbuf.size()
	buffer := make([]byte, rbufsize+wbufsize)
	tunnel := &packetConnTunnel[T]{
		refc: 2,
		sock: s,
		conn: conn,
		errs: errs,
	}

	go tunnel.readFromPacketConn(buffer[:rbufsize])
	go tunnel.writeToPacketConn(buffer[rbufsize:])
	go tunnel.closeOnCancel(ctx)
	return wasi.ESUCCESS
}

func (s *socket[T]) allocateBuffersIfNil() {
	if s.rbuf == nil {
		s.rbuf = newSocketBuffer[T](s.rev.lock, int(s.rbufsize))
		s.rev = &s.rbuf.rev
	}
	if s.wbuf == nil {
		s.wbuf = newSocketBuffer[T](s.wev.lock, int(s.wbufsize))
		s.wev = &s.wbuf.wev
	}
}

func (s *socket[T]) SockConnect(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	raddr, errno := s.net.sockaddr(addr)
	if errno != wasi.ESUCCESS {
		return errno
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.flags.has(sockClosed) {
		return wasi.ECONNRESET
	}

	// Stream sockets cannot be connected if they are listening, nor reconnected
	// if a connection has already been initiated.
	if s.typ == stream {
		if s.flags.has(sockListen) {
			return wasi.EISCONN
		}
		if s.flags.has(sockConn) {
			return wasi.EALREADY
		}
	}

	if errno := s.bindToNetwork(ctx); errno != wasi.ESUCCESS {
		return errno
	}

	s.allocateBuffersIfNil()
	s.flags = s.flags.with(sockConn)

	if s.typ == datagram {
		s.raddr = raddr
		s.wev.trigger()
		return wasi.ESUCCESS
	}

	// At most three errors are produced to this channel, one when the dial
	// function failed or the connection is blocking, and up to two if both
	// the read and write pipes error.
	errs := make(chan wasi.Errno, 3)
	s.errs = errs

	blocking := !s.flags.has(sockNonBlock)
	if s.net.contains(raddr) {
		if sock := s.net.socket(netaddr[T]{s.proto, raddr}); sock == nil {
			errs <- wasi.ECONNREFUSED
		} else if _, errno := sock.connect(s, raddr, s.laddr); errno != wasi.ESUCCESS {
			errs <- errno
		} else {
			s.raddr = raddr
		}
		s.wev.trigger()
		close(errs)
	} else {
		ctx, s.cancel = context.WithCancel(ctx)
		go func() {
			upstream, errno := s.net.dial(ctx, s.proto, s.raddr)
			if errno != wasi.ESUCCESS || blocking {
				errs <- errno
			}

			s.wev.trigger()

			if errno != wasi.ESUCCESS {
				close(errs)
				return
			}

			downstream := newHostConn(s)
			rbufsize := s.rbuf.size()
			wbufsize := s.wbuf.size()
			buffer := make([]byte, rbufsize+wbufsize)
			tunnel := &connTunnel{
				refc:  2,
				conn1: upstream,
				conn2: downstream,
				errs:  errs,
			}

			go tunnel.copy(upstream, downstream, buffer[:rbufsize])
			go tunnel.copy(downstream, upstream, buffer[rbufsize:])
			go closeReadOnCancel(ctx, upstream)
		}()
	}

	if !blocking {
		return wasi.EINPROGRESS
	}

	select {
	case errno := <-errs:
		return errno
	case <-ctx.Done():
		return wasi.MakeErrno(ctx.Err())
	}
}

func (s *socket[T]) SockRecv(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	size, roflags, _, errno := s.sockRecvFrom(ctx, iovs, flags)
	return size, roflags, errno
}

func (s *socket[T]) SockRecvFrom(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	size, roflags, addr, errno := s.sockRecvFrom(ctx, iovs, flags)
	if errno != wasi.ESUCCESS {
		return size, roflags, nil, errno
	}
	return size, roflags, addr.sockAddr(), wasi.ESUCCESS
}

func (s *socket[T]) sockRecvFrom(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (size wasi.Size, roflags wasi.ROFlags, addr T, errno wasi.Errno) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.flags.has(sockClosed) {
		return ^wasi.Size(0), 0, addr, wasi.ECONNRESET
	}
	if s.flags.has(sockListen) {
		return ^wasi.Size(0), 0, addr, wasi.ENOTSUP
	}
	if s.typ == stream && !s.flags.has(sockConn) {
		return ^wasi.Size(0), 0, addr, wasi.ENOTCONN
	}
	if flags.Has(wasi.RecvWaitAll) {
		return ^wasi.Size(0), 0, addr, wasi.ENOTSUP
	}
	if errno := s.getErrno(); errno != wasi.ESUCCESS {
		return ^wasi.Size(0), 0, addr, errno
	}
	s.allocateBuffersIfNil()

	for {
		size, roflags, addr, errno := s.recvmsg(s.rbuf, iovs, flags)
		// Connected sockets may receive packets from other sockets that they
		// are not connected to (e.g. when using datagram sockets and sendto),
		// so we drop the packages here when that's the case and move to read
		// the next packet.
		switch errno {
		case wasi.ESUCCESS:
			if size == 0 || !s.flags.has(sockConn) || addr == s.raddr {
				return size, roflags, addr, errno
			}
		case wasi.EAGAIN:
			if s.flags.has(sockNonBlock) {
				return size, roflags, addr, errno
			}
			if errno := wait(ctx, &s.mutex, s.rev, s.rtimeout, s.poll); errno != wasi.ESUCCESS {
				return size, roflags, addr, errno
			}
		default:
			return size, roflags, addr, errno
		}
	}
}

func (s *socket[T]) SockSend(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	if !s.flags.has(sockConn) {
		return ^wasi.Size(0), wasi.ENOTCONN
	}
	return s.sockSendTo(ctx, iovs, flags, s.raddr)
}

func (s *socket[T]) SockSendTo(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	dstAddr, errno := s.net.sockaddr(addr)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	return s.sockSendTo(ctx, iovs, flags, dstAddr)
}

func (s *socket[T]) sockSendTo(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags, addr T) (wasi.Size, wasi.Errno) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.flags.has(sockClosed) {
		return ^wasi.Size(0), wasi.ECONNRESET
	}
	if s.flags.has(sockListen) {
		return ^wasi.Size(0), wasi.ENOTSUP
	}
	if s.typ == stream && !s.flags.has(sockConn) {
		return ^wasi.Size(0), wasi.ENOTCONN
	}
	if s.flags.has(sockConn) && addr != s.raddr {
		return ^wasi.Size(0), wasi.EISCONN
	}
	if errno := s.getErrno(); errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	if errno := s.bindToAny(ctx); errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	s.allocateBuffersIfNil()

	var sbuf *sockbuf[T]
	var sev *event
	var smu *sync.Mutex

	if s.typ == stream || !s.net.contains(addr) {
		sbuf = s.wbuf
		sev = s.wev
		smu = &s.mutex
	} else {
		size := sumIOVecLen(iovs)
		// When the destination is a datagram socket on the same network we can
		// send the datagram directly to its receive buffer, bypassing the need
		// to copy the data between socket buffers.
		if size > s.wbuf.size() {
			return 0, wasi.EMSGSIZE
		}
		sock := s.net.socket(netaddr[T]{s.proto, addr})
		if sock == nil || sock.typ != datagram {
			return wasi.Size(size), wasi.ESUCCESS
		}

		// We're sending the packet to a different socket so there is no need to
		// hold the current socket's mutex anymore. Also prevent deadlock if the
		// other peer was concurrently trying to synchronize on the receiver to
		// send it a packet.
		s.mutex.Unlock()
		defer s.mutex.Lock()

		sock.synchronize(func() {
			sock.allocateBuffersIfNil()
			sbuf = sock.rbuf
			sev = &sock.rbuf.wev
			smu = &sock.mutex
		})
	}

	for {
		n, errno := s.sendmsg(sbuf, iovs, s.laddr)
		// Messages that are too large to fit in the socket buffer are dropped
		// since this may only happen on datagram sockets which are lossy links.
		switch errno {
		case wasi.ESUCCESS:
			return n, errno
		case wasi.EMSGSIZE:
			return wasi.Size(sumIOVecLen(iovs)), wasi.ESUCCESS
		case wasi.EAGAIN:
			if s.flags.has(sockNonBlock) {
				return n, errno
			}
			if errno := wait(ctx, smu, sev, s.wtimeout, s.poll); errno != wasi.ESUCCESS {
				return n, errno
			}
		default:
			return n, errno
		}
	}
}

func (s *socket[T]) SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return s.laddr.sockAddr(), wasi.ESUCCESS
}

func (s *socket[T]) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return s.raddr.sockAddr(), wasi.ESUCCESS
}

func (s *socket[T]) SockGetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
	case wasi.RecvBufferSize:
		return wasi.IntValue(s.rbufsize), wasi.ESUCCESS
	case wasi.SendBufferSize:
		return wasi.IntValue(s.wbufsize), wasi.ESUCCESS
	case wasi.KeepAlive:
	case wasi.OOBInline:
	case wasi.Linger:
	case wasi.RecvLowWatermark:
	case wasi.RecvTimeout:
		return wasi.TimeValue(s.rtimeout), wasi.ESUCCESS
	case wasi.SendTimeout:
		return wasi.TimeValue(s.wtimeout), wasi.ESUCCESS
	case wasi.QueryAcceptConnections:
	case wasi.BindToDevice:
	}
	return nil, wasi.EINVAL
}

func (s *socket[T]) SockSetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	s.mutex.Lock()
	defer s.mutex.Unlock()

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
	case wasi.RecvBufferSize:
		return setIntValueLimit(&s.rbufsize, value, minSocketBufferSize, maxSocketBufferSize)
	case wasi.SendBufferSize:
		return setIntValueLimit(&s.wbufsize, value, minSocketBufferSize, maxSocketBufferSize)
	case wasi.KeepAlive:
	case wasi.OOBInline:
	case wasi.Linger:
	case wasi.RecvLowWatermark:
	case wasi.RecvTimeout:
		return setDurationValue(&s.rtimeout, value)
	case wasi.SendTimeout:
		return setDurationValue(&s.wtimeout, value)
	case wasi.QueryAcceptConnections:
	case wasi.BindToDevice:
	}
	return wasi.EINVAL
}

func setIntValueLimit(option *int32, value wasi.SocketOptionValue, minval, maxval int32) wasi.Errno {
	if v, ok := value.(wasi.IntValue); ok {
		if value := int32(v); value >= 0 {
			if value < minval {
				value = minval
			}
			if value > maxval {
				value = maxval
			}
			*option = value
			return wasi.ESUCCESS
		}
	}
	return wasi.EINVAL
}

func setDurationValue(option *time.Duration, value wasi.SocketOptionValue) wasi.Errno {
	if v, ok := value.(wasi.TimeValue); ok {
		if duration := time.Duration(v); duration >= 0 {
			*option = duration
			return wasi.ESUCCESS
		}
	}
	return wasi.EINVAL
}

func (s *socket[T]) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	if (flags & (wasi.ShutdownRD | wasi.ShutdownWR)) == 0 {
		return wasi.EINVAL
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.flags.has(sockClosed) || !s.flags.has(sockConn) {
		return wasi.ENOTCONN
	}

	if flags.Has(wasi.ShutdownRD) {
		if s.rbuf == nil || s.rbuf.rev.state() == aborted {
			return wasi.ENOTCONN
		}
		s.rbuf.close()
	}

	if flags.Has(wasi.ShutdownWR) {
		if s.wbuf == nil || s.wbuf.wev.state() == aborted {
			return wasi.ENOTCONN
		}
		s.wbuf.close()
	}

	return wasi.ESUCCESS
}

func (s *socket[T]) FDClose(ctx context.Context) wasi.Errno {
	s.close()
	return wasi.ESUCCESS
}

func (s *socket[T]) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	s.mutex.Lock()
	s.flags = s.flags.withFDFlags(flags)
	s.mutex.Unlock()
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
