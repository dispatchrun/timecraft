package sandbox

import (
	"context"
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
			return 0, os.ErrDeadlineExceeded
		}

		n, _, _, errno := c.rbuf.recv([]wasi.IOVec{b})
		if errno == wasi.ESUCCESS {
			if n == 0 {
				return 0, io.EOF
			}
			return int(n), nil
		}
		if errno != wasi.EAGAIN {
			return int(n), errno // TODO: convert to syscall error
		}

		var ready bool
		c.rev.synchronize(func() { ready = c.rev.poll(c.rpoll) })

		if !ready {
			select {
			case <-c.rpoll:
			case <-c.done:
				return 0, io.EOF
			case <-c.rdeadline.channel():
				return 0, os.ErrDeadlineExceeded
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
			return n, os.ErrDeadlineExceeded
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
			return n, errno // TODO: convert to syscall error
		}

		var ready bool
		c.wev.synchronize(func() { ready = c.wev.poll(c.wpoll) })

		if !ready {
			select {
			case <-c.wpoll:
			case <-c.done:
				return n, io.EOF
			case <-c.wdeadline.channel():
				return n, os.ErrDeadlineExceeded
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
		return net.ErrClosed
	default:
		c.rdeadline.set(t)
		c.wdeadline.set(t)
		return nil
	}
}

func (c *conn[T]) SetReadDeadline(t time.Time) error {
	select {
	case <-c.done:
		return net.ErrClosed
	default:
		c.rdeadline.set(t)
		return nil
	}
}

func (c *conn[T]) SetWriteDeadline(t time.Time) error {
	select {
	case <-c.done:
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

type connPipe struct {
	refc  int32
	conn1 net.Conn
	conn2 net.Conn
	errs  chan<- wasi.Errno
}

func (c *connPipe) unref() {
	if atomic.AddInt32(&c.refc, -1) == 0 {
		c.conn1.Close()
		c.conn2.Close()
		close(c.errs)
	}
}

func (c *connPipe) copy(dst, src net.Conn, buf []byte) {
	defer c.unref()
	defer closeWrite(dst)
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

func closeRead(conn net.Conn) error {
	switch c := conn.(type) {
	case closeReader:
		return c.CloseRead()
	default:
		return conn.Close()
	}
}

func closeWrite(conn net.Conn) error {
	switch c := conn.(type) {
	case closeWriter:
		return c.CloseWrite()
	default:
		return c.Close()
	}
}

func closeReadOnCancel(ctx context.Context, conn net.Conn) {
	<-ctx.Done()
	closeRead(conn)
}
