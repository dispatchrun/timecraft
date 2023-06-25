package sandbox_test

import (
	"context"
	"net"
	"testing"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"golang.org/x/net/nettest"
)

func TestConn(t *testing.T) {
	tests := []struct {
		network string
		address string
		options []sandbox.Option
	}{
		{
			network: "tcp4",
			address: "127.0.0.1:0",
		},

		{
			network: "tcp6",
			address: "[::1]:0",
		},

		{
			network: "unix",
			address: "unix.sock",
			options: []sandbox.Option{
				sandbox.Socket("unix.sock"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.network, func(t *testing.T) {
			nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
				ctx := context.Background()
				sys := sandbox.New(test.options...)

				l, err := sys.Listen(ctx, test.network, test.address)
				if err != nil {
					return nil, nil, nil, err
				}

				connChan := make(chan net.Conn, 1)
				errChan := make(chan error, 1)
				go func() {
					c, err := l.Accept()
					if err != nil {
						errChan <- err
					} else {
						connChan <- c
					}
				}()

				addr := l.Addr()
				c1, err = sys.Dial(ctx, addr.Network(), addr.String())
				if err != nil {
					l.Close()
					return nil, nil, nil, err
				}
				select {
				case c2 = <-connChan:
				case err = <-errChan:
					c1.Close()
					l.Close()
					return nil, nil, nil, err
				}

				if err := l.Close(); err != nil {
					c1.Close()
					c2.Close()
					return nil, nil, nil, err
				}

				stop = func() { c1.Close(); c2.Close(); sys.Close(ctx) }
				return c1, c2, stop, nil
			})
		})
	}
}

func TestPacketConn(t *testing.T) {
	tests := []struct {
		network string
		address string
		options []sandbox.Option
	}{
		{
			network: "udp4",
			address: "127.0.0.1:0",
		},

		{
			network: "udp6",
			address: "[::1]:0",
		},
	}

	for _, test := range tests {
		t.Run(test.network, func(t *testing.T) {
			nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
				ctx := context.Background()
				sys := sandbox.New(test.options...)

				l, err := sys.ListenPacket(ctx, test.network, test.address)
				if err != nil {
					return nil, nil, nil, err
				}

				addr := l.LocalAddr()
				c, err := sys.Dial(ctx, addr.Network(), addr.String())
				if err != nil {
					l.Close()
					return nil, nil, nil, err
				}

				c1 = &connectedPacketConn{
					PacketConn: l,
					peer:       c.(net.PacketConn),
				}

				c2 = &connectedPacketConn{
					PacketConn: c.(net.PacketConn),
					peer:       l,
				}

				stop = func() { c1.Close(); c2.Close(); sys.Close(ctx) }
				return c1, c2, stop, nil
			})
		})
	}
}

type connectedPacketConn struct {
	net.PacketConn
	peer net.PacketConn
}

func (c *connectedPacketConn) Close() error {
	if cr, ok := c.peer.(interface{ CloseRead() error }); ok {
		cr.CloseRead()
	}
	return c.PacketConn.Close()
}

func (c *connectedPacketConn) Read(b []byte) (int, error) {
	n, _, err := c.ReadFrom(b)
	return n, err
}

func (c *connectedPacketConn) Write(b []byte) (int, error) {
	return c.WriteTo(b, c.peer.LocalAddr())
}

func (c *connectedPacketConn) RemoteAddr() net.Addr {
	return c.peer.LocalAddr()
}
