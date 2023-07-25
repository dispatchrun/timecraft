package sandbox_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"golang.org/x/sys/unix"
)

func findNonLoopbackIPv4HostAddress() ([]net.Addr, error) {
	addrs, err := findNonLoopbackHostAddresses()
	if err != nil {
		return nil, err
	}
	var ipv4Addrs []net.Addr
	for _, addr := range addrs {
		switch a := addr.(type) {
		case *net.IPNet:
			if a.IP.To4() != nil {
				ipv4Addrs = append(ipv4Addrs, &net.IPAddr{IP: a.IP})
			}
		}
	}
	if len(ipv4Addrs) == 0 {
		return nil, fmt.Errorf("no IPv4 addresses were found that were not on the loopback interface")
	}
	return ipv4Addrs, nil
}

func findNonLoopbackHostAddresses() ([]net.Addr, error) {
	ifaces, err := sandbox.Host().Interfaces()
	if err != nil {
		return nil, err
	}
	var nonLoopbackAddrs []net.Addr
	for _, iface := range ifaces {
		flags := iface.Flags()
		if (flags & net.FlagUp) == 0 {
			continue
		}
		if (flags & net.FlagLoopback) != 0 {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}
		nonLoopbackAddrs = append(nonLoopbackAddrs, addrs...)
	}
	if len(nonLoopbackAddrs) == 0 {
		return nil, fmt.Errorf("no addresses were found that were not on the loopback interface")
	}
	return nonLoopbackAddrs, nil
}

func testNamespaceConnectStreamLoopbackIPv4(t *testing.T, ns sandbox.Namespace) {
	testNamespaceConnectStream(t, ns, &sandbox.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
	})
}

func testNamespaceConnectStreamLoopbackIPv6(t *testing.T, ns sandbox.Namespace) {
	testNamespaceConnectStream(t, ns, &sandbox.SockaddrInet6{
		Addr: [16]byte{15: 1},
	})
}

func testNamespaceConnectStream(t *testing.T, ns sandbox.Namespace, bind sandbox.Sockaddr) {
	family := sandbox.SockaddrFamily(bind)

	server, err := ns.Socket(family, sandbox.STREAM, sandbox.TCP)
	assert.OK(t, err)
	defer server.Close()
	assert.OK(t, server.SetNonBlock(true))

	assert.OK(t, server.Bind(bind))
	assert.OK(t, server.Listen(1))
	serverAddr, err := server.Name()
	assert.OK(t, err)

	client, err := ns.Socket(family, sandbox.STREAM, sandbox.TCP)
	assert.OK(t, err)
	defer client.Close()
	assert.OK(t, client.SetNonBlock(true))

	assert.Error(t, client.Connect(serverAddr), sandbox.EINPROGRESS)
	assert.OK(t, waitReadyRead(server))
	conn, addr, err := server.Accept()
	assert.OK(t, err)
	defer conn.Close()
	assert.OK(t, conn.SetNonBlock(true))

	assert.OK(t, waitReadyWrite(client))
	peer, err := client.Peer()
	assert.OK(t, err)
	assert.Equal(t, sandbox.SockaddrAddrPort(peer), sandbox.SockaddrAddrPort(serverAddr))

	name, err := client.Name()
	assert.OK(t, err)
	assert.DeepEqual(t, name, addr)

	wn, err := client.SendTo([][]byte{[]byte("Hello, World!")}, nil, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 13)

	assert.OK(t, waitReadyRead(conn))

	buf := make([]byte, 32)
	rn, rflags, peer, err := conn.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 13)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:13]), "Hello, World!")
	assert.Equal(t, peer, nil)
}

func testNamespaceConnectDatagramLoopbackIPv4(t *testing.T, ns sandbox.Namespace) {
	testNamespaceConnectDatagram(t, ns, &sandbox.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
	})
}

func testNamespaceConnectDatagramLoopbackIPv6(t *testing.T, ns sandbox.Namespace) {
	testNamespaceConnectDatagram(t, ns, &sandbox.SockaddrInet6{
		Addr: [16]byte{15: 1},
	})
}

func testNamespaceConnectDatagram(t *testing.T, ns sandbox.Namespace, bind sandbox.Sockaddr) {
	family := sandbox.SockaddrFamily(bind)

	server, err := ns.Socket(family, sandbox.DGRAM, sandbox.UDP)
	assert.OK(t, err)
	defer server.Close()

	assert.OK(t, server.Bind(bind))
	addr, err := server.Name()
	assert.OK(t, err)

	client, err := ns.Socket(family, sandbox.DGRAM, sandbox.UDP)
	assert.OK(t, err)
	defer client.Close()

	assert.OK(t, client.Connect(addr))

	name, err := client.Name()
	assert.OK(t, err)
	assert.NotEqual(t, name, nil)

	peer, err := client.Peer()
	assert.OK(t, err)
	assert.Equal(t, sandbox.SockaddrAddrPort(peer), sandbox.SockaddrAddrPort(addr))

	wn, err := client.SendTo([][]byte{[]byte("Hello, World!")}, nil, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 13)

	buf := make([]byte, 32)
	rn, rflags, peer, err := server.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 13)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:13]), "Hello, World!")
	assert.Equal(t, sandbox.SockaddrAddrPort(peer), sandbox.SockaddrAddrPort(name))

	wn, err = server.SendTo([][]byte{[]byte("How are you?")}, peer, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 12)

	rn, rflags, peer, err = client.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 12)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:12]), "How are you?")
	assert.Equal(t, sandbox.SockaddrAddrPort(peer), sandbox.SockaddrAddrPort(addr))
}

func testNamespaceExchangeDatagramLoopbackIPv4(t *testing.T, ns sandbox.Namespace) {
	testNamespaceExchangeDatagram(t, ns, &sandbox.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
	})
}

func testNamespaceExchangeDatagramLoopbackIPv6(t *testing.T, ns sandbox.Namespace) {
	testNamespaceExchangeDatagram(t, ns, &sandbox.SockaddrInet6{
		Addr: [16]byte{15: 1},
	})
}

func testNamespaceExchangeDatagram(t *testing.T, ns sandbox.Namespace, bind sandbox.Sockaddr) {
	family := sandbox.SockaddrFamily(bind)

	socket1, err := ns.Socket(family, sandbox.DGRAM, sandbox.UDP)
	assert.OK(t, err)
	defer socket1.Close()

	assert.OK(t, socket1.Bind(bind))
	addr1, err := socket1.Name()
	assert.OK(t, err)

	socket2, err := ns.Socket(family, sandbox.DGRAM, sandbox.UDP)
	assert.OK(t, err)
	defer socket2.Close()

	assert.OK(t, socket2.Bind(bind))
	addr2, err := socket2.Name()
	assert.OK(t, err)

	wn, err := socket1.SendTo([][]byte{[]byte("Hello, World!")}, addr2, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 13)

	buf := make([]byte, 32)

	rn, rflags, addr, err := socket2.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 13)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:13]), "Hello, World!")
	assert.Equal(t, sandbox.SockaddrAddrPort(addr), sandbox.SockaddrAddrPort(addr1))

	wn, err = socket2.SendTo([][]byte{[]byte("How are you?")}, addr1, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 12)

	rn, rflags, addr, err = socket1.RecvFrom([][]byte{buf[:11]}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 11)
	assert.Equal(t, rflags, sandbox.TRUNC)
	assert.Equal(t, string(buf[:11]), "How are you")
	assert.Equal(t, sandbox.SockaddrAddrPort(addr), sandbox.SockaddrAddrPort(addr2))

	wn, err = socket1.SendTo([][]byte{[]byte("How are you?")}, addr, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 12)

	rn, rflags, addr, err = socket2.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 12)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:12]), "How are you?")
	assert.Equal(t, sandbox.SockaddrAddrPort(addr), sandbox.SockaddrAddrPort(addr1))
}

func waitReadyRead(socket sandbox.Socket) error {
	return wait(socket, unix.POLLIN, time.Second)
}

func waitReadyWrite(socket sandbox.Socket) error {
	return wait(socket, unix.POLLOUT, time.Second)
}

func wait(socket sandbox.Socket, events int16, timeout time.Duration) error {
	tms := int(timeout / time.Millisecond)
	pfd := []unix.PollFd{{
		Fd:     int32(socket.Fd()),
		Events: events,
	}}
	for {
		_, err := unix.Poll(pfd, tms)
		if err != unix.EINTR {
			return err
		}
	}
}
