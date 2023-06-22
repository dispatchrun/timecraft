package sandbox

import (
	"io"
	"net"
	"os"
	"sync"
	"time"

	"github.com/stealthrocket/wasi-go"
)

/*
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
*/

type conn[T sockaddr] struct {
	socket *socket[T]
	laddr  net.Addr
	raddr  net.Addr

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
		rbuf:      socket.rbuf,
		wbuf:      socket.wbuf,
		rev:       socket.rev,
		wev:       socket.wev,
		laddr:     socket.laddr.netAddr(socket.proto),
		raddr:     socket.raddr.netAddr(socket.proto),
		rdeadline: makeDeadline(),
		wdeadline: makeDeadline(),
		rpoll:     make(chan struct{}),
		wpoll:     make(chan struct{}),
		done:      make(chan struct{}),
	}
}

func (c *conn[T]) swap() {
	c.rbuf, c.wbuf = c.wbuf, c.rbuf
	c.rev, c.wev = c.wev, c.rev
}

func (c *conn[T]) Close() error {
	c.socket.close()
	c.rdeadline.set(time.Time{})
	c.wdeadline.set(time.Time{})
	c.once.Do(func() { close(c.done) })
	return nil
}

func (c *conn[T]) Read(b []byte) (int, error) {
	c.rmu.Lock()
	defer c.rmu.Unlock()
	for {
		if c.rdeadline.expired() {
			return 0, os.ErrDeadlineExceeded
		}

		if !c.rev.poll(c.rpoll) {
			select {
			case <-c.rpoll:
			case <-c.done:
				return 0, io.EOF
			case <-c.rdeadline.channel():
				return 0, os.ErrDeadlineExceeded
			}
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
	}
}

func (c *conn[T]) Write(b []byte) (int, error) {
	c.wmu.Lock()
	defer c.wmu.Unlock()

	var n int
	for {
		if c.wdeadline.expired() {
			return n, os.ErrDeadlineExceeded
		}

		if !c.wev.poll(c.wpoll) {
			select {
			case <-c.wpoll:
			case <-c.done:
				return n, io.EOF
			case <-c.wdeadline.channel():
				return n, os.ErrDeadlineExceeded
			}
		}

		r, errno := c.wbuf.send([]wasi.IOVec{b}, c.socket.laddr)
		n += int(r)
		if errno == wasi.ESUCCESS {
			if int(r) < len(b) {
				b = b[r:]
				continue
			}
			return n, nil
		}
		if errno != wasi.EAGAIN {
			return n, errno // TODO: convert to syscall error
		}
	}
}

func (c *conn[T]) LocalAddr() net.Addr {
	return c.laddr
}

func (c *conn[T]) RemoteAddr() net.Addr {
	return c.raddr
}

func (c *conn[T]) SetDeadline(t time.Time) error {
	c.rdeadline.set(t)
	c.wdeadline.set(t)
	return nil
}

func (c *conn[T]) SetReadDeadline(t time.Time) error {
	c.rdeadline.set(t)
	return nil
}

func (c *conn[T]) SetWriteDeadline(t time.Time) error {
	c.wdeadline.set(t)
	return nil
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
