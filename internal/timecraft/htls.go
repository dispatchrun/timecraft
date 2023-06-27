package timecraft

import (
	"context"
	"crypto/tls"
	"net"
	"time"

	"github.com/stealthrocket/wasi-go"
)

const htlsHostnameOption = 0x9000 + 1

// NewHTLSSystem creates a wasi.System that offloads TLS operations to the host.
func NewHTLSSystem(system wasi.System) wasi.System {
	return &htlsSystem{System: system}
}

type htlsSystem struct {
	wasi.System
	conns map[wasi.FD]*tls.Conn
}

func (s *htlsSystem) SockSetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	if level != wasi.ReservedLevel {
		return s.System.SockSetOpt(ctx, fd, level, option, value)
	}
	if option != htlsHostnameOption {
		return wasi.EINVAL
	}
	if s.conns == nil {
		s.conns = make(map[wasi.FD]*tls.Conn)
	}
	if _, exists := s.conns[fd]; exists {
		panic("fd in htls table already exists")
	}
	s.conns[fd] = tls.Client(&wasiConn{sys: s.System, fd: fd}, &tls.Config{
		ServerName: string(value.(wasi.StringValue)),
	})
	return wasi.ESUCCESS
}

func (s *htlsSystem) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	c, ok := s.conns[fd]
	if !ok {
		return s.System.SockSend(ctx, fd, iovecs, flags)
	}
	n, err := c.Write(iovecs[0])
	if err != nil {
		return wasi.Size(n), wasi.MakeErrno(err)
	}
	return wasi.Size(n), wasi.ESUCCESS
}

func (s *htlsSystem) SockRecv(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	c, ok := s.conns[fd]
	if !ok {
		return s.System.SockRecv(ctx, fd, iovecs, flags)
	}
	n, err := c.Read(iovecs[0])
	if err != nil {
		return wasi.Size(n), 0, wasi.MakeErrno(err)
	}
	return wasi.Size(n), 0, wasi.ESUCCESS
}

func (s *htlsSystem) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	c, ok := s.conns[fd]
	if !ok {
		return s.System.SockShutdown(ctx, fd, flags)
	}
	err := c.Close()
	delete(s.conns, fd)
	return wasi.MakeErrno(err)
}

func (s *htlsSystem) FDClose(ctx context.Context, fd wasi.FD) wasi.Errno {
	c, ok := s.conns[fd]
	if !ok {
		return s.System.FDClose(ctx, fd)
	}
	err := c.Close()
	delete(s.conns, fd)
	return wasi.MakeErrno(err)
}

func (s *htlsSystem) Close(ctx context.Context) error {
	var err error
	for _, c := range s.conns {
		cerr := c.Close()
		if cerr != nil {
			err = cerr
		}
	}
	cerr := s.System.Close(ctx)
	if cerr != nil {
		err = cerr
	}
	s.conns = nil
	return err
}

// implements net.Conn over a wasi system
type wasiConn struct {
	sys wasi.System
	fd  wasi.FD

	readdl  time.Time
	writedl time.Time
}

func (c *wasiConn) Read(b []byte) (n int, err error) {
	ctx := context.Background()
	if !c.readdl.IsZero() {
		ctx2, cancel := context.WithDeadline(ctx, c.readdl)
		ctx = ctx2
		defer cancel()
	}

	iovecs := []wasi.IOVec{b}
	// TODO roflags
	s, _, errno := c.sys.SockRecv(ctx, c.fd, iovecs, 0)
	if errno != wasi.ESUCCESS {
		return int(s), errno
	}
	return int(s), nil
}

func (c *wasiConn) Write(b []byte) (n int, err error) {
	ctx := context.Background()
	if !c.writedl.IsZero() {
		ctx2, cancel := context.WithDeadline(ctx, c.writedl)
		ctx = ctx2
		defer cancel()
	}
	iovecs := []wasi.IOVec{b}
	// TODO roflags
	s, errno := c.sys.SockSend(ctx, c.fd, iovecs, 0)
	if errno != wasi.ESUCCESS {
		return int(s), errno
	}
	return int(s), nil
}

func (c *wasiConn) Close() error {
	return nil
}

func (c *wasiConn) LocalAddr() net.Addr {
	panic("TODO LocalAddr")
}

func (c *wasiConn) RemoteAddr() net.Addr {
	panic("TODO RemoteAddr")
}

func (c *wasiConn) SetDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	if err != nil {
		return err
	}
	return c.SetWriteDeadline(t)
}

func (c *wasiConn) SetReadDeadline(t time.Time) error {
	c.readdl = t
	return nil
}

func (c *wasiConn) SetWriteDeadline(t time.Time) error {
	c.writedl = t
	return nil
}
