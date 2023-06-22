package sandbox

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/stealthrocket/wasi-go"
)

type packetConn[T sockaddr] struct {
	net       network[T]
	proto     protocol
	laddr     T
	raddr     T
	rdeadline deadline
	wdeadline deadline
	once      sync.Once
	done      chan struct{}
}

func newPacketConn[T sockaddr](socket *socket[T]) *packetConn[T] {
	return &packetConn[T]{
		net:       socket.net,
		proto:     socket.proto,
		laddr:     socket.laddr,
		raddr:     socket.raddr,
		rdeadline: makeDeadline(),
		wdeadline: makeDeadline(),
		done:      make(chan struct{}),
	}
}

func (c *packetConn[T]) Close() error {
	c.once.Do(func() { close(c.done) })
	c.rdeadline.set(time.Time{})
	c.wdeadline.set(time.Time{})
	return nil
}

func (c *packetConn[T]) Read(b []byte) (int, error) {
	s := c.net.socket(netaddr[T]{c.proto, c.raddr})
	if s == nil {
		return 0, syscall.ECONNREFUSED
	}

	pipe := s.send
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if c.rdeadline.expired() {
		return 0, os.ErrDeadlineExceeded
	}

	select {
	case pipe.ch <- b:
		pipe.ev.trigger()
		return len(b) - len(<-pipe.ch), nil
	case <-c.rdeadline.channel():
		return 0, os.ErrDeadlineExceeded
	case <-c.done:
		return 0, io.EOF
	}
}

func (c *packetConn[T]) Write(b []byte) (n int, err error) {
	s := c.net.socket(netaddr[T]{c.proto, c.raddr})
	if s == nil {
		return len(b), nil
	}

	pipe := s.recv
	pipe.mu.Lock()
	defer pipe.mu.Unlock()

	if c.wdeadline.expired() {
		return 0, os.ErrDeadlineExceeded
	}

	select {
	case pipe.ch <- b:
		pipe.ev.trigger()
		return len(b) - len(<-pipe.ch), nil
	case <-c.wdeadline.channel():
		return n, os.ErrDeadlineExceeded
	case <-c.done:
		return n, io.ErrClosedPipe
	}
}

func (c *packetConn[T]) LocalAddr() net.Addr {
	return c.laddr.netAddr(c.proto)
}

func (c *packetConn[T]) RemoteAddr() net.Addr {
	return c.raddr.netAddr(c.proto)
}

func (c *packetConn[T]) SetDeadline(t time.Time) error {
	return errors.Join(c.SetReadDeadline(t), c.SetWriteDeadline(t))
}

func (c *packetConn[T]) SetReadDeadline(t time.Time) error {
	select {
	case <-c.done:
		return net.ErrClosed
	default:
		c.rdeadline.set(t)
		return nil
	}
}

func (c *packetConn[T]) SetWriteDeadline(t time.Time) error {
	select {
	case <-c.done:
		return net.ErrClosed
	default:
		c.wdeadline.set(t)
		return nil
	}
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
