package timecraft

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"net"
	"time"

	"github.com/stealthrocket/wasi-go"
)

func NewHTLSSystem(system wasi.System) wasi.System {
	return &htlsSystem{System: system}
}

type htlsSystem struct {
	wasi.System

	conns map[wasi.FD]*conn
	buf   []byte // 4 bytes
}

type conn struct {
	name []byte
	conn *tls.Conn
}

func (s *htlsSystem) SockSetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	if level != 255 {
		return s.System.SockSetOpt(ctx, fd, level, option, value)
	}

	v := value.(wasi.IntValue)

	// option values need to match wasi_socket_ext.c
	switch option {
	case 1:
		if s.conns == nil {
			s.conns = make(map[wasi.FD]*conn)
		}
		if _, exists := s.conns[fd]; exists {
			panic("fd in htls table already exists")
		}
		s.conns[fd] = &conn{
			name: make([]byte, 0, v),
		}
	case 2:
		c, ok := s.conns[fd]
		if !ok {
			return wasi.EINVAL
		}
		left := cap(c.name) - len(c.name)
		r := 4
		if left < r {
			r = left
		}

		if cap(s.buf) < 4 {
			s.buf = make([]byte, 4)
		}
		binary.LittleEndian.PutUint32(s.buf, uint32(v))
		c.name = append(c.name, s.buf[:r]...)

		if cap(c.name) == len(c.name) {
			fmt.Println("ESTABLISHED HTLS ON SOCKET", fd, "TO:", string(c.name))
		}
	}

	return 0
}

func (s *htlsSystem) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	c, ok := s.conns[fd]
	if !ok {
		return s.System.SockSend(ctx, fd, iovecs, flags)
	}

	// First action on the socket since the socket options were transmitted.
	// Time to set up TLS on the connection.
	if c.conn == nil {
		if cap(c.name) != len(c.name) {
			panic(fmt.Errorf("client hasn't sent the full hostname with setsockopt: %d/%d bytes", len(c.name), cap(c.name)))
		}

		c.conn = tls.Client(wasiConn{s.System, fd}, &tls.Config{
			ServerName: string(c.name),
		})
	}

	n, err := c.conn.Write(iovecs[0])
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

	if c.conn == nil {
		panic("client should have tried to write on the connection first")
	}

	n, err := c.conn.Read(iovecs[0])
	if err != nil {
		return wasi.Size(n), 0, wasi.MakeErrno(err)
	}
	return wasi.Size(n), 0, wasi.ESUCCESS
}

// implements net.Conn over a wasi system
type wasiConn struct {
	sys wasi.System
	fd  wasi.FD
}

func (c wasiConn) Read(b []byte) (n int, err error) {
	ctx := context.TODO()
	iovecs := []wasi.IOVec{b}
	// TODO roflags
	s, _, errno := c.sys.SockRecv(ctx, c.fd, iovecs, 0)
	if errno != wasi.ESUCCESS {
		return int(s), errno
	}
	return int(s), nil
}

func (c wasiConn) Write(b []byte) (n int, err error) {
	ctx := context.TODO()
	iovecs := []wasi.IOVec{b}
	// TODO roflags
	s, errno := c.sys.SockSend(ctx, c.fd, iovecs, 0)
	if errno != wasi.ESUCCESS {
		return int(s), errno
	}
	return int(s), nil
}

func (c wasiConn) Close() error {
	ctx := context.TODO()
	errno := c.sys.SockShutdown(ctx, c.fd, 0)
	if errno != wasi.ESUCCESS {
		return errno
	}
	return nil
}

func (c wasiConn) LocalAddr() net.Addr {
	panic("TODO LocalAddr")
}

func (c wasiConn) RemoteAddr() net.Addr {
	panic("TODO RemoteAddr")
}

func (c wasiConn) SetDeadline(t time.Time) error {
	panic("TODO setdeadline")
}

func (c wasiConn) SetReadDeadline(t time.Time) error {
	panic("TODO setreaddeadline")
}

func (c wasiConn) SetWriteDeadline(t time.Time) error {
	panic("TODO setwritedeadline")
}
