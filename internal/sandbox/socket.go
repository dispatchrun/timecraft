//nolint:unused
package sandbox

import (
	"context"
	"sync"

	"github.com/stealthrocket/wasi-go"
)

type packet[T sockaddr] struct {
	addr T
	size wasi.Size
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

func (sb *sockbuf[T]) size() wasi.Size {
	sb.mu.Lock()
	size := sb.buf.cap()
	sb.mu.Unlock()
	return wasi.Size(size)
}

func (sb *sockbuf[T]) recv(iovs []wasi.IOVec) (size wasi.Size, roflags wasi.ROFlags, addr T, errno wasi.Errno) {
	sb.lock()
	defer sb.unlock()

	if sb.src.len() == 0 {
		if sb.rev.state() == aborted {
			return 0, 0, addr, wasi.ESUCCESS
		}
		return ^wasi.Size(0), 0, addr, wasi.EAGAIN
	}

	for _, iov := range iovs {
		size += wasi.Size(len(iov))
	}

	packet := sb.src.index(0)
	addr = packet.addr
	if packet.size < size {
		size = packet.size
	}

	remain := size
	for _, iov := range iovs {
		if remain < wasi.Size(len(iov)) {
			iov = iov[:remain]
		}
		n := sb.buf.read(iov)
		if remain -= wasi.Size(n); remain == 0 {
			break
		}
	}

	if packet.size -= size; packet.size == 0 {
		sb.src.discard(1)
	}
	return size, 0, addr, wasi.ESUCCESS
}

func (sb *sockbuf[T]) recvmsg(iovs []wasi.IOVec) (size wasi.Size, flags wasi.ROFlags, addr T, errno wasi.Errno) {
	sb.lock()
	defer sb.unlock()

	if sb.src.len() == 0 {
		if sb.rev.state() == aborted {
			return 0, 0, addr, wasi.ESUCCESS
		}
		return ^wasi.Size(0), 0, addr, wasi.EAGAIN
	}

	for _, iov := range iovs {
		size += wasi.Size(len(iov))
	}

	packet := sb.src.index(0)
	addr = packet.addr
	switch {
	case packet.size < size:
		size = packet.size
	case packet.size > size:
		flags |= wasi.RecvDataTruncated
	}

	remain := size
	for _, iov := range iovs {
		if remain < wasi.Size(len(iov)) {
			iov = iov[:remain]
		}
		n := sb.buf.read(iov)
		if remain -= wasi.Size(n); remain == 0 {
			break
		}
	}

	sb.buf.discard(int(packet.size - size))
	sb.src.discard(1)
	return size, flags, addr, wasi.ESUCCESS
}

func (sb *sockbuf[T]) send(iovs []wasi.IOVec, addr T) (size wasi.Size, errno wasi.Errno) {
	sb.lock()
	defer sb.unlock()

	if sb.buf.avail() == 0 {
		if sb.wev.state() == aborted {
			errno = wasi.ENOTCONN
		} else {
			errno = wasi.EAGAIN
		}
		return ^wasi.Size(0), errno
	}

	for _, iov := range iovs {
		n := sb.buf.write(iov)
		size += wasi.Size(n)
		if n < len(iov) {
			break
		}
	}

	sb.src.append(packet[T]{
		addr: addr,
		size: size,
	})
	return size, wasi.ESUCCESS
}

func (sb *sockbuf[T]) sendmsg(iovs []wasi.IOVec, addr T) (size wasi.Size, errno wasi.Errno) {
	sb.lock()
	defer sb.unlock()

	if sb.wev.state() == aborted {
		return ^wasi.Size(0), wasi.ENOTCONN
	}
	if sb.buf.avail() == 0 {
		return ^wasi.Size(0), wasi.EAGAIN
	}

	for _, iov := range iovs {
		size += wasi.Size(len(iov))
	}

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
	return size, wasi.ESUCCESS
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
	flags wasi.FDFlags
	// Events indicating readiness for read or write operations on the socket.
	rev *event
	wev *event
	// For listening sockets, this event and channel are used to pass
	// connections between the two ends of the network.
	mutex  sync.Mutex
	accept chan *socket[T]
	// Connected sockets have a bidirectional pipe that links between the two
	// ends of the socket pair.
	rbuf *sockbuf[T] // socket recv end (guest side)
	wbuf *sockbuf[T] // socket send end (guest side)
	// Functions used to send and receive message on the socket, implement the
	// difference in behavior between stream and datagram sockets.
	sendmsg func(*sockbuf[T], []wasi.IOVec, T) (wasi.Size, wasi.Errno)
	recvmsg func(*sockbuf[T], []wasi.IOVec) (wasi.Size, wasi.ROFlags, T, wasi.Errno)
	// This field is set to true if the socket was created by the host, which
	// allows listeners to detect if they are accepting connections from the
	// guest, and construct the right connection type.
	host bool
	// A boolean used to indicate that this is a connected socket.
	conn bool
	// A boolean used to indicate that this is a listening socket.
	listen bool
	// A boolean indicating whether the socket has been closed. Closing may
	// happen asynchronously so the expectation is for the methods to acquire
	// the mutex lock before reading this value.
	closed bool
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
}

const (
	defaultSocketBufferSize = 16384
	minSocketBufferSize     = 1024
	maxSocketBufferSize     = 65536
)

func newSocket[T sockaddr](net network[T], typ socktype, proto protocol, lock *sync.Mutex) *socket[T] {
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
	closed := s.closed
	cancel := s.cancel
	s.closed = true
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
	sock := newSocket[T](s.net, s.typ, s.proto, lock)
	sock.conn = true
	sock.laddr = laddr
	sock.raddr = raddr

	if peer == nil {
		sock.host = true
		sock.wbuf = newSocketBuffer[T](lock, int(sock.wbufsize))
		sock.rbuf = newSocketBuffer[T](lock, int(sock.rbufsize))
	} else {
		// Sockets paired on the same network share the same receive and send
		// buffers, but swapped so data sent by the peer is read by the socket
		// vice versa.
		sock.rbuf = peer.wbuf
		sock.wbuf = peer.rbuf
		sock.rev = &sock.rbuf.rev
		sock.wev = &sock.wbuf.wev
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.accept == nil || s.closed {
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

	if s.closed || s.conn {
		return wasi.EINVAL
	}

	if !s.listen {
		if errno := s.bindToAny(); errno != wasi.ESUCCESS {
			return errno
		}
		s.accept = make(chan *socket[T], backlog)
		s.listen = true
	}

	return wasi.ESUCCESS
}

func (s *socket[T]) SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno) {
	if s.typ == datagram {
		return nil, wasi.ENOTSUP
	}
	if !s.flags.Has(wasi.NonBlock) {
		return nil, wasi.ENOTSUP
	}

	s.mutex.Lock()
	accept := s.accept
	closed := s.closed
	s.mutex.Unlock()

	if accept == nil || closed {
		return nil, wasi.EINVAL
	}

	s.rev.clear()
	select {
	case sock := <-accept:
		if sock == nil {
			// This condition may occur when the socket is closed concurrently,
			// which closes the accept channel and results in receiving nil
			// values without blocking.
			return nil, wasi.EINVAL
		}
		if len(accept) > 0 {
			s.rev.trigger()
		}
		sock.flags = flags
		return sock, wasi.ESUCCESS
	default:
		return nil, wasi.EAGAIN
	}
}

func (s *socket[T]) SockBind(ctx context.Context, bind wasi.SocketAddress) wasi.Errno {
	addr, errno := s.net.sockaddr(bind)
	if errno != wasi.ESUCCESS {
		return errno
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	var zero T
	if s.closed || s.laddr != zero {
		return wasi.EINVAL
	}
	return s.net.bind(addr, s)
}

func (s *socket[T]) bindToAny() wasi.Errno {
	var zero T
	if s.laddr != zero {
		return wasi.ESUCCESS
	}
	return s.net.bind(zero, s)
}

func (s *socket[T]) bindToNetwork() wasi.Errno {
	var zero T
	if s.laddr != zero {
		return wasi.ESUCCESS
	}
	return s.net.link(s)
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

	if s.closed {
		return wasi.ECONNRESET
	}

	// Stream sockets cannot be connected if they are listening, nor reconnected
	// if a connection has already been initiated.
	if s.typ == stream {
		if s.listen {
			return wasi.EISCONN
		}
		if s.conn {
			return wasi.EALREADY
		}
	}

	if errno := s.bindToNetwork(); errno != wasi.ESUCCESS {
		return errno
	}

	s.allocateBuffersIfNil()
	// At most three errors are produced to this channel, one when the dial
	// function failed or the connection is blocking, and up to two if both
	// the read and write pipes error.
	errs := make(chan wasi.Errno, 3)
	s.errs = errs
	s.conn = true

	if s.typ == datagram {
		s.raddr = raddr
		s.wev.trigger()
		close(errs)
		return wasi.ESUCCESS
	}

	blocking := !s.flags.Has(wasi.NonBlock)
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
			upstream, errno := s.net.dial(ctx, s.proto, s.laddr, s.raddr)
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
			pipe := &connPipe{refc: 2, conn1: upstream, conn2: downstream, errs: errs}
			go pipe.copy(upstream, downstream, buffer[:rbufsize])
			go pipe.copy(downstream, upstream, buffer[rbufsize:])
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

	// TODO:
	// - RecvPeek
	// - RecvWaitAll
	if s.closed {
		return ^wasi.Size(0), 0, addr, wasi.ECONNRESET
	}
	if s.listen {
		return ^wasi.Size(0), 0, addr, wasi.ENOTSUP
	}
	if s.typ == stream && !s.conn {
		return ^wasi.Size(0), 0, addr, wasi.ENOTCONN
	}
	if flags.Has(wasi.RecvWaitAll) {
		return ^wasi.Size(0), 0, addr, wasi.ENOTSUP
	}
	if flags.Has(wasi.RecvPeek) {
		return ^wasi.Size(0), 0, addr, wasi.ENOTSUP
	}
	if !s.flags.Has(wasi.NonBlock) {
		return ^wasi.Size(0), 0, addr, wasi.ENOTSUP
	}
	if errno := s.getErrno(); errno != wasi.ESUCCESS {
		return ^wasi.Size(0), 0, addr, errno
	}

	s.allocateBuffersIfNil()
	for {
		size, roflags, addr, errno := s.recvmsg(s.rbuf, iovs)
		// Connected sockets may receive packets from other sockets that they
		// are not connected to (e.g. when using datagram sockets and sendto),
		// so we drop the packages here when that's the case and move to read
		// the next packet.
		if size == 0 && errno == wasi.ESUCCESS { // EOF
			return size, roflags, addr, errno
		}
		if errno != wasi.ESUCCESS { // ERROR
			return size, roflags, addr, errno
		}
		if !s.conn || addr == s.raddr { // packet from unconnected socket
			return size, roflags, addr, errno
		}
	}
}

func (s *socket[T]) SockSend(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	if !s.conn {
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

	if s.closed {
		return ^wasi.Size(0), wasi.ECONNRESET
	}
	if s.listen {
		return ^wasi.Size(0), wasi.ENOTSUP
	}
	if s.typ == stream && !s.conn {
		return ^wasi.Size(0), wasi.ENOTCONN
	}
	if s.conn && addr != s.raddr {
		return ^wasi.Size(0), wasi.EISCONN
	}
	if !s.flags.Has(wasi.NonBlock) {
		return ^wasi.Size(0), wasi.ENOTSUP
	}

	if errno := s.getErrno(); errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	if errno := s.bindToAny(); errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	s.allocateBuffersIfNil()

	var sbuf *sockbuf[T]
	var size wasi.Size
	if s.typ == stream {
		sbuf = s.wbuf
	} else {
		for _, iov := range iovs {
			size += wasi.Size(len(iov))
		}
		if size > s.wbuf.size() {
			return 0, wasi.EMSGSIZE
		}
		sock := s.net.socket(netaddr[T]{s.proto, addr})
		if sock == nil {
			return size, wasi.ESUCCESS
		}
		sock.allocateBuffersIfNil()
		sbuf = sock.rbuf
	}
	n, errno := s.sendmsg(sbuf, iovs, s.laddr)
	// Messages that are too large to fit in the socket buffer are dropped
	// since this may only happen on datagram sockets which are lossy links.
	if errno == wasi.EMSGSIZE {
		return size, wasi.ESUCCESS
	}
	return n, errno
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
	case wasi.SendBufferSize:
		return wasi.IntValue(s.wbufsize), wasi.ESUCCESS
	case wasi.RecvBufferSize:
		return wasi.IntValue(s.rbufsize), wasi.ESUCCESS
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
	case wasi.SendBufferSize:
		return setIntValueLimit(&s.wbufsize, value, minSocketBufferSize, maxSocketBufferSize)
	case wasi.RecvBufferSize:
		return setIntValueLimit(&s.rbufsize, value, minSocketBufferSize, maxSocketBufferSize)
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

func (s *socket[T]) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	if (flags & (wasi.ShutdownRD | wasi.ShutdownWR)) == 0 {
		return wasi.EINVAL
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.closed || !s.conn {
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
	s.flags = flags
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
