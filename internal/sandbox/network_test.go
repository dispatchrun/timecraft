package sandbox_test

import (
	"context"
	"net"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"golang.org/x/net/nettest"
)

func TestNetwork(t *testing.T) {
	tests := []struct {
		network string
		address string
		options []sandbox.Option
	}{
		{
			network: "tcp4",
			address: "127.0.0.1:80",
		},

		{
			network: "tcp6",
			address: "[::1]:80",
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
				c1, err = sys.Connect(ctx, addr.Network(), addr.String())
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

func TestSystemListenPortZero(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New()

	lstn, err := sys.Listen(ctx, "tcp", "127.0.0.1:0")
	assert.OK(t, err)

	addr, ok := lstn.Addr().(*net.TCPAddr)
	assert.True(t, ok)
	assert.True(t, addr.IP.Equal(net.IPv4(127, 0, 0, 1)))
	assert.NotEqual(t, addr.Port, 0)
	assert.OK(t, lstn.Close())
}

func TestSystemListenAnyAddress(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New()

	lstn, err := sys.Listen(ctx, "tcp", ":4242")
	assert.OK(t, err)

	addr, ok := lstn.Addr().(*net.TCPAddr)
	assert.True(t, ok)
	assert.True(t, addr.IP.Equal(net.IPv4(127, 0, 0, 1)))
	assert.Equal(t, addr.Port, 4242)
	assert.OK(t, lstn.Close())
}
