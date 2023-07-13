package network_test

import (
	"net"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/network"
)

func TestVirtualNetwork(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T, *network.VirtualNetwork)
	}{
		{
			scenario: "a virtual network namespace has two interfaces",
			function: testVirtualNetworkInterfaces,
		},

		{
			scenario: "ipv4 sockets can connect to one another on a loopback interface",
			function: testVirtualNetworkConnectLoopbackIPv4,
		},

		{
			scenario: "ipv6 sockets can connect to one another on a loopback interface",
			function: testVirtualNetworkConnectLoopbackIPv6,
		},

		{
			scenario: "ipv4 sockets can connect to one another on a network interface",
			function: testVirtualNetworkConnectInterfaceIPv4,
		},

		{
			scenario: "ipv6 sockets can connect to one another on a network interface",
			function: testVirtualNetworkConnectInterfaceIPv6,
		},

		{
			scenario: "ipv4 sockets in different namespaces can connet to one another",
			function: testVirtualNetworkConnectNamespacesIPv4,
		},

		{
			scenario: "ipv6 sockets in different namespaces can connet to one another",
			function: testVirtualNetworkConnectNamespacesIPv6,
		},
	}

	_, ipnet4, _ := net.ParseCIDR("192.168.0.0/24")
	_, ipnet6, _ := net.ParseCIDR("fe80::/64")

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			test.function(t,
				network.NewVirtualNetwork(network.Host(), ipnet4, ipnet6),
			)
		})
	}
}

func testVirtualNetworkInterfaces(t *testing.T, n *network.VirtualNetwork) {
	ns, err := n.CreateNamespace()
	assert.OK(t, err)

	ifaces, err := ns.Interfaces()
	assert.OK(t, err)
	assert.Equal(t, len(ifaces), 2)

	lo0 := ifaces[0]
	assert.Equal(t, lo0.Index(), 0)
	assert.Equal(t, lo0.MTU(), 1500)
	assert.Equal(t, lo0.Name(), "lo0")
	assert.Equal(t, lo0.Flags(), net.FlagUp|net.FlagLoopback)

	lo0Addrs, err := lo0.Addrs()
	assert.OK(t, err)
	assert.Equal(t, len(lo0Addrs), 2)
	assert.Equal(t, lo0Addrs[0].String(), "127.0.0.1")
	assert.Equal(t, lo0Addrs[1].String(), "::1")

	en0 := ifaces[1]
	assert.Equal(t, en0.Index(), 1)
	assert.Equal(t, en0.MTU(), 1500)
	assert.Equal(t, en0.Name(), "en0")
	assert.Equal(t, en0.Flags(), net.FlagUp)

	en0Addrs, err := en0.Addrs()
	assert.OK(t, err)
	assert.Equal(t, len(en0Addrs), 2)
	assert.Equal(t, en0Addrs[0].String(), "192.168.0.1")
	assert.Equal(t, en0Addrs[1].String(), "fe80::1")
}

func testVirtualNetworkConnectLoopbackIPv4(t *testing.T, n *network.VirtualNetwork) {
	testVirtualNetworkConnect(t, n, &network.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
		Port: 80,
	})
}

func testVirtualNetworkConnectLoopbackIPv6(t *testing.T, n *network.VirtualNetwork) {
	testVirtualNetworkConnect(t, n, &network.SockaddrInet6{
		Addr: [16]byte{15: 1},
		Port: 80,
	})
}

func testVirtualNetworkConnectInterfaceIPv4(t *testing.T, n *network.VirtualNetwork) {
	testVirtualNetworkConnect(t, n, &network.SockaddrInet4{
		Addr: [4]byte{192, 168, 0, 1},
		Port: 80,
	})
}

func testVirtualNetworkConnectInterfaceIPv6(t *testing.T, n *network.VirtualNetwork) {
	testVirtualNetworkConnect(t, n, &network.SockaddrInet6{
		Addr: [16]byte{0: 0xfe, 1: 0x80, 15: 1},
		Port: 80,
	})
}

func testVirtualNetworkConnect(t *testing.T, n *network.VirtualNetwork, bind network.Sockaddr) {
	family := network.SockaddrFamily(bind)

	ns, err := n.CreateNamespace()
	assert.OK(t, err)

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
	assert.DeepEqual(t, peer, serverAddr)

	name, err := client.Name()
	assert.OK(t, err)
	assert.DeepEqual(t, name, addr)

	wn, err := client.SendTo([][]byte{[]byte("Hello, World!")}, nil, nil, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 13)

	assert.OK(t, waitReadyRead(conn))
	buf := make([]byte, 32)
	rn, oobn, rflags, peer, err := conn.RecvFrom([][]byte{buf}, nil, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 13)
	assert.Equal(t, oobn, 0)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:13]), "Hello, World!")
	assert.Equal(t, peer, addr)
}

func testVirtualNetworkConnectNamespacesIPv4(t *testing.T, n *network.VirtualNetwork) {
	testVirtualNetworkConnectNamespaces(t, n, network.INET)
}

func testVirtualNetworkConnectNamespacesIPv6(t *testing.T, n *network.VirtualNetwork) {
	testVirtualNetworkConnectNamespaces(t, n, network.INET6)
}

func testVirtualNetworkConnectNamespaces(t *testing.T, n *network.VirtualNetwork, family network.Family) {
	ns1, err := n.CreateNamespace()
	assert.OK(t, err)

	ns2, err := n.CreateNamespace()
	assert.OK(t, err)

	server, err := ns1.Socket(family, network.STREAM, network.TCP)
	assert.OK(t, err)
	defer server.Close()

	assert.OK(t, server.Listen(1))
	serverAddr, err := server.Name()
	assert.OK(t, err)

	client, err := ns2.Socket(family, network.STREAM, network.TCP)
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
	assert.DeepEqual(t, peer, serverAddr)

	name, err := client.Name()
	assert.OK(t, err)
	assert.DeepEqual(t, name, addr)

	wn, err := client.SendTo([][]byte{[]byte("Hello, World!")}, nil, nil, 0)
	assert.OK(t, err)
	assert.Equal(t, wn, 13)

	assert.OK(t, waitReadyRead(conn))
	buf := make([]byte, 32)
	rn, oobn, rflags, peer, err := conn.RecvFrom([][]byte{buf}, nil, 0)
	assert.OK(t, err)
	assert.Equal(t, rn, 13)
	assert.Equal(t, oobn, 0)
	assert.Equal(t, rflags, 0)
	assert.Equal(t, string(buf[:13]), "Hello, World!")
	assert.Equal(t, peer, addr)
}

func waitReadyRead(socket network.Socket) error {
	return network.WaitReadyRead(socket, time.Second)
}

func waitReadyWrite(socket network.Socket) error {
	return network.WaitReadyWrite(socket, time.Second)
}
