package sandbox_test

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"testing"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"golang.org/x/net/nettest"
)

func TestConn(t *testing.T) {
	tests := []struct {
		network string
		address string
	}{
		{
			network: "tcp4",
			address: ":0",
		},

		{
			network: "tcp6",
			address: "[::]:0",
		},

		{
			network: "unix",
			address: "unix.sock",
		},
	}

	for _, test := range tests {
		t.Run(test.network, func(t *testing.T) {
			nettest.TestConn(t, func() (c1, c2 net.Conn, stop func(), err error) {
				network := test.network
				address := test.address

				switch network {
				case "unix":
					address = filepath.Join(t.TempDir(), address)
				}

				localNet := sandbox.NewLocalNetwork()
				localNs, err := localNet.CreateNamespace(sandbox.Host())
				if err != nil {
					t.Fatal(err)
				}

				ctx := context.Background()
				sys := sandbox.New(sandbox.Network(localNs))

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
	// Note: this is not as thorough of a test as TestConn because UDP is lossy
	// and building a net.Conn on top of a net.PacketConn causes tests to fail
	// due to packet losses.
	tests := []struct {
		network string
		address string
	}{
		{
			network: "udp",
			address: ":0",
		},
		{
			network: "udp4",
			address: ":0",
		},
		{
			network: "udp6",
			address: "[::]:0",
		},
	}

	for _, test := range tests {
		t.Run(test.network, func(t *testing.T) {
			network := test.network
			address := test.address

			switch network {
			case "unixgram":
				address = filepath.Join(t.TempDir(), address)
			}

			// localNet := sandbox.NewLocalNetwork()
			// localNs, err := localNet.CreateNamespace(sandbox.Host())
			// if err != nil {
			// 	t.Fatal(err)
			// }

			ctx := context.Background()
			sys := sandbox.New(sandbox.Network(sandbox.Host()))

			c1, err := sys.ListenPacket(ctx, network, address)
			if err != nil {
				t.Fatal(err)
			}
			defer c1.Close()
			addr := c1.LocalAddr()

			c, err := sys.Dial(ctx, addr.Network(), addr.String())
			if err != nil {
				fmt.Println(err)
				t.Fatal(err)
			}
			c2 := c.(net.PacketConn)
			defer c2.Close()

			rb2 := make([]byte, 128)
			wb := []byte("PACKETCONN TEST")

			if n, err := c1.WriteTo(wb, c2.LocalAddr()); err != nil {
				t.Fatal(err)
			} else if n != len(wb) {
				t.Fatalf("write with wrong number of bytes: want=%d got=%d", len(wb), n)
			}

			if n, addr, err := c2.ReadFrom(rb2); err != nil {
				t.Fatal(err)
			} else if n != len(wb) {
				t.Fatalf("read with wrong number of bytes: want=%d got=%d", len(wb), n)
			} else if !addrPortEqual(addr, c1.LocalAddr()) {
				t.Fatalf("read from wrong address: want=%s got=%s", c1.LocalAddr(), addr)
			}

			if n, err := c.Write(wb); err != nil {
				t.Fatal(err)
			} else if n != len(wb) {
				t.Fatalf("write with wrong number of bytes: want=%d got=%d", len(wb), n)
			}

			rb1 := make([]byte, 128)
			if n, addr, err := c1.ReadFrom(rb1); err != nil {
				t.Fatal(err)
			} else if n != len(wb) {
				t.Fatalf("read with wrong number of bytes: want=%d got=%d", len(wb), n)
			} else if !addrPortEqual(addr, c2.LocalAddr()) {
				t.Fatalf("read from wrong address: want=%s got=%s", c2.LocalAddr(), addr)
			}
		})
	}
}

func addrPortEqual(addr1, addr2 net.Addr) bool {
	switch a1 := addr1.(type) {
	case *net.UDPAddr:
		if a2, ok := addr2.(*net.UDPAddr); ok {
			return a1.Port == a2.Port
		}
	}
	return false
}
