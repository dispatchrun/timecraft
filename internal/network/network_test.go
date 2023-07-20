package network_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/network"
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
	ifaces, err := network.Host().Interfaces()
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

func testNamespaceConnectStreamLoopbackIPv4(t *testing.T, ns network.Namespace) {
	testNamespaceConnectStream(t, ns, &network.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
	})
}

func testNamespaceConnectStreamLoopbackIPv6(t *testing.T, ns network.Namespace) {
	testNamespaceConnectStream(t, ns, &network.SockaddrInet6{
		Addr: [16]byte{15: 1},
	})
}

func testNamespaceConnectStream(t *testing.T, ns network.Namespace, bind network.Sockaddr) {
	family := network.SockaddrFamily(bind)

	server, err := ns.Socket(family, network.STREAM, network.TCP)
	assert.OK(t, err)
	defer server.Close()

	assert.OK(t, server.Bind(bind))
	assert.OK(t, server.Listen(1))
	serverAddr, err := server.Name()
	assert.OK(t, err)

	client, err := ns.Socket(family, network.STREAM, network.TCP)
	assert.OK(t, err)
	defer client.Close()

	assert.Error(t, client.Connect(serverAddr), network.EINPROGRESS)
	assert.OK(t, waitReadyRead(server))
	conn, addr, err := server.Accept()
	assert.OK(t, err)
	defer conn.Close()

	assert.OK(t, waitReadyWrite(client))
	peer, err := client.Peer()
	assert.OK(t, err)
	assert.Equal(t, network.SockaddrAddrPort(peer), network.SockaddrAddrPort(serverAddr))

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

func testNamespaceConnectDatagramLoopbackIPv4(t *testing.T, ns network.Namespace) {
	testNamespaceConnectDatagram(t, ns, &network.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
	})
}

func testNamespaceConnectDatagramLoopbackIPv6(t *testing.T, ns network.Namespace) {
	testNamespaceConnectDatagram(t, ns, &network.SockaddrInet6{
		Addr: [16]byte{15: 1},
	})
}

func testNamespaceConnectDatagram(t *testing.T, ns network.Namespace, bind network.Sockaddr) {
	family := network.SockaddrFamily(bind)

	server, err := ns.Socket(family, network.DGRAM, network.UDP)
	assert.OK(t, err)
	defer server.Close()

	assert.OK(t, server.Bind(bind))
	addr, err := server.Name()
	assert.OK(t, err)

	client, err := ns.Socket(family, network.DGRAM, network.UDP)
	assert.OK(t, err)
	defer client.Close()

	assert.OK(t, client.Connect(addr))
	assert.OK(t, waitReadyWrite(client))

	name, err := client.Name()
	assert.OK(t, err)
	assert.NotEqual(t, name, nil)

	peer, err := client.Peer()
	assert.OK(t, err)
	assert.Equal(t, network.SockaddrAddrPort(peer), network.SockaddrAddrPort(addr))

	wn, err := client.SendTo([][]byte{[]byte("Hello, World!")}, nil, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 13)
	assert.OK(t, waitReadyRead(server))

	buf := make([]byte, 32)
	rn, rflags, peer, err := server.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 13)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:13]), "Hello, World!")
	assert.Equal(t, network.SockaddrAddrPort(peer), network.SockaddrAddrPort(name))

	wn, err = server.SendTo([][]byte{[]byte("How are you?")}, peer, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 12)
	assert.OK(t, waitReadyRead(client))

	rn, rflags, peer, err = client.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 12)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:12]), "How are you?")
	assert.Equal(t, network.SockaddrAddrPort(peer), network.SockaddrAddrPort(addr))
}

func testNamespaceExchangeDatagramLoopbackIPv4(t *testing.T, ns network.Namespace) {
	testNamespaceExchangeDatagram(t, ns, &network.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
	})
}

func testNamespaceExchangeDatagramLoopbackIPv6(t *testing.T, ns network.Namespace) {
	testNamespaceExchangeDatagram(t, ns, &network.SockaddrInet6{
		Addr: [16]byte{15: 1},
	})
}

func testNamespaceExchangeDatagram(t *testing.T, ns network.Namespace, bind network.Sockaddr) {
	family := network.SockaddrFamily(bind)

	socket1, err := ns.Socket(family, network.DGRAM, network.UDP)
	assert.OK(t, err)
	defer socket1.Close()

	assert.OK(t, socket1.Bind(bind))
	addr1, err := socket1.Name()
	assert.OK(t, err)

	socket2, err := ns.Socket(family, network.DGRAM, network.UDP)
	assert.OK(t, err)
	defer socket2.Close()

	assert.OK(t, socket2.Bind(bind))
	addr2, err := socket2.Name()
	assert.OK(t, err)

	wn, err := socket1.SendTo([][]byte{[]byte("Hello, World!")}, addr2, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 13)

	assert.OK(t, waitReadyRead(socket2))
	buf := make([]byte, 32)

	rn, rflags, addr, err := socket2.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 13)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:13]), "Hello, World!")
	assert.Equal(t, network.SockaddrAddrPort(addr), network.SockaddrAddrPort(addr1))

	wn, err = socket2.SendTo([][]byte{[]byte("How are you?")}, addr1, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 12)

	assert.OK(t, waitReadyRead(socket1))

	rn, rflags, addr, err = socket1.RecvFrom([][]byte{buf[:11]}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 11)
	assert.Equal(t, rflags, network.TRUNC)
	assert.Equal(t, string(buf[:11]), "How are you")
	assert.Equal(t, network.SockaddrAddrPort(addr), network.SockaddrAddrPort(addr2))

	wn, err = socket1.SendTo([][]byte{[]byte("How are you?")}, addr, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 12)
	assert.OK(t, waitReadyRead(socket2))

	rn, rflags, addr, err = socket2.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 12)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:12]), "How are you?")
	assert.Equal(t, network.SockaddrAddrPort(addr), network.SockaddrAddrPort(addr1))
}

func waitReadyRead(socket network.Socket) error {
	return network.WaitReadyRead(socket, time.Second)
}

func waitReadyWrite(socket network.Socket) error {
	return network.WaitReadyWrite(socket, time.Second)
}
