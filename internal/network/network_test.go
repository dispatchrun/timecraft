package network_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/network"
)

func testNamespaceConnectLoopbackIPv4(t *testing.T, ns network.Namespace) {
	testNamespaceConnect(t, ns, &network.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
	})
}

func testNamespaceConnectLoopbackIPv6(t *testing.T, ns network.Namespace) {
	testNamespaceConnect(t, ns, &network.SockaddrInet6{
		Addr: [16]byte{15: 1},
	})
}

func testNamespaceConnect(t *testing.T, ns network.Namespace, bind network.Sockaddr) {
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
}
