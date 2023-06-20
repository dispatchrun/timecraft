package sandbox

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/stealthrocket/wasi-go"
)

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
	// When the socket receives an asynchronous error, it keeps track of it in
	// this field to support getsockopt and prevent future attempts to read or
	// write on it.
	errno wasi.Errno
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
		net:    net,
		typ:    typ,
		proto:  proto,
		send:   newPipe(lock),
		recv:   newPipe(lock),
		cancel: func() {},
		// socket options
		recvBufferSize: defaultSocketBufferSize,
		sendBufferSize: defaultSocketBufferSize,
	}
}

func (s *socket[T]) close() {
	_ = s.net.unlink(s)
	s.recv.close()
	s.send.close()
	s.cancel()
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
	if backlog < 0 {
		return wasi.EINVAL
	}
	if backlog == 0 || backlog > 128 {
		backlog = 128
	}
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.accept == nil {
		var zero T
		if s.laddr == zero {
			return wasi.EDESTADDRREQ
		}
		s.accept = make(chan *socket[T], backlog)
	}
	return wasi.ESUCCESS
}

func (s *socket[T]) SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno) {
	var sock *socket[T]
	s.recv.ev.clear()

	if s.recv.flags.Has(wasi.NonBlock) {
		select {
		case sock = <-s.accept:
		case <-s.recv.done:
			return nil, wasi.EBADF
		default:
			return nil, wasi.EAGAIN
		}
	} else {
		select {
		case sock = <-s.accept:
		case <-s.recv.done:
			return nil, wasi.EBADF
		case <-ctx.Done():
			return nil, wasi.MakeErrno(ctx.Err())
		}
	}

	if len(s.accept) > 0 {
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
	if s.accept != nil {
		return wasi.EISCONN
	}
	if s.conn {
		return wasi.EALREADY
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

	ctx, s.cancel = context.WithCancel(ctx)
	// At most two errors are produced to this channel, either one when the dial
	// function failed, or up to two if both the read and write pipes error.
	errs := make(chan wasi.Errno, 2)
	s.errs = errs
	s.conn = true

	blocking := !s.send.flags.Has(wasi.NonBlock)
	recvBufferSize := s.recvBufferSize
	sendBufferSize := s.sendBufferSize
	go func() {
		var conn net.Conn
		var errno wasi.Errno

		if !s.net.contains(s.raddr) {
			conn, errno = s.net.dial(ctx, s.proto, s.laddr, s.raddr)
		} else {
			sock := s.net.socket(netaddr[T]{s.proto, s.raddr})
			if sock == nil {
				errno = wasi.ECONNREFUSED
			} else {
				sock, errno = sock.connect(s.raddr, s.laddr, false)
				if errno == wasi.ESUCCESS {
					conn = newHostConn(sock)
				}
			}
		}

		if errno != wasi.ESUCCESS || blocking {
			errs <- errno
		}

		// Trigger the notification that the connection has been established
		// and the program can expect to read and write on the socket, or get
		// the error if the connection failed.
		s.send.ev.trigger()

		if errno != wasi.ESUCCESS {
			close(errs)
			return
		}

		// TODO: pool the buffers?
		sockbuf := make([]byte, recvBufferSize+sendBufferSize)
		group := sync.WaitGroup{}
		group.Add(2)

		go func() {
			defer group.Done()
			copySocketPipe(errs, s.send, conn, outputReadCloser{s.send}, sockbuf[:recvBufferSize])
		}()

		go func() {
			defer group.Done()
			copySocketPipe(errs, s.recv, inputWriteCloser{s.recv}, conn, sockbuf[recvBufferSize:])
		}()

		go func() {
			group.Wait()
			conn.Close()
			close(errs)
		}()
	}()

	if !blocking {
		return wasi.EINPROGRESS
	}

	select {
	case errno = <-errs:
	case <-ctx.Done():
		errno = wasi.MakeErrno(ctx.Err())
	}
	s.errno = errno
	return errno
}

func copySocketPipe(errs chan<- wasi.Errno, p *pipe, w io.Writer, r io.Reader, b []byte) {
	_, err := io.CopyBuffer(w, r, b)
	if err != nil {
		errs <- wasi.MakeErrno(err)
	}
	p.close()
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

func (s *socket[T]) SockSend(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	if errno := s.getErrno(); errno != wasi.ESUCCESS {
		return 0, errno
	}
	return s.send.ch.write(ctx, iovs, s.send.flags, &s.send.ev, nil, s.send.done)
}

func (s *socket[T]) SockSendTo(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	if s.accept != nil {
		return 0, wasi.ENOTSUP
	}
	if s.conn {
		return 0, wasi.EISCONN
	}
	return 0, wasi.ENOSYS // TODO: implement sentto
}

func (s *socket[T]) SockRecvFrom(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	if s.accept != nil {
		return 0, 0, nil, wasi.ENOTSUP
	}
	if s.conn {
		return 0, 0, nil, wasi.EISCONN
	}
	return 0, 0, nil, wasi.ENOSYS // TODO: implement recvfrom
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
		return wasi.IntValue(s.errno), wasi.ESUCCESS
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
	return nil, wasi.ENOPROTOOPT
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
		if s.conn {
			return wasi.EISCONN
		}
		return setIntValueLimit(&s.sendBufferSize, value, minSocketBufferSize, maxSocketBufferSize)
	case wasi.RecvBufferSize:
		if s.conn {
			return wasi.EISCONN
		}
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
	return wasi.ENOPROTOOPT
}

func setIntValueLimit(option *int32, value wasi.SocketOptionValue, minval, maxval int32) wasi.Errno {
	switch v := value.(type) {
	case wasi.IntValue:
		v32 := int32(v)
		if v32 < minval {
			v32 = minval
		}
		if v32 > maxval {
			v32 = maxval
		}
		*option = v32
		return wasi.ESUCCESS
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
	s.send.FDClose(ctx)
	s.recv.FDClose(ctx)

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
	case errno, ok := <-s.errs:
		if ok {
			s.errno = errno
		}
	default:
	}
	return s.errno
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
		laddr:     socket.laddr.netAddr(socket.proto),
		raddr:     socket.raddr.netAddr(socket.proto),
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

	pipe.ev.trigger()
	select {
	case pipe.ch <- b:
		return len(b) - len(<-pipe.ch), nil
	case <-c.rdeadline.channel():
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
		pipe.ev.trigger()
		select {
		case pipe.ch <- b[n:]:
			n = len(b) - len(<-pipe.ch)
		case <-c.wdeadline.channel():
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
		laddr:     socket.raddr.netAddr(socket.proto),
		raddr:     socket.laddr.netAddr(socket.proto),
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
	timeout := c.rdeadline.channel()
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
	timeout := c.wdeadline.channel()
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
