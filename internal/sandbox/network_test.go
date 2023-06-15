package sandbox_test

import (
	"context"
	"net"
	"testing"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"golang.org/x/net/nettest"
)

func TestNetwork(t *testing.T) {
	nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
		ctx := context.Background()
		sys := sandbox.New()

		network := "tcp"
		address := "127.0.0.1:80"

		l, err := sys.Listen(ctx, network, address)
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

		c1, err = sys.Connect(ctx, network, address)
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
}
