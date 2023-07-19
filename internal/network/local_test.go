package network_test

import (
	"context"
	"io"
	"net"
	"net/netip"
	"strconv"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/network"
)

func TestLocalNetwork(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T, *network.LocalNetwork)
	}{
		{
			scenario: "a local network namespace has two interfaces",
			function: testLocalNetworkInterfaces,
		},

		{
			scenario: "ipv4 sockets can connect to one another on a loopback interface",
			function: testLocalNetworkConnectLoopbackIPv4,
		},

		{
			scenario: "ipv6 sockets can connect to one another on a loopback interface",
			function: testLocalNetworkConnectLoopbackIPv6,
		},

		{
			scenario: "ipv4 sockets can connect to one another on a network interface",
			function: testLocalNetworkConnectInterfaceIPv4,
		},

		{
			scenario: "ipv6 sockets can connect to one another on a network interface",
			function: testLocalNetworkConnectInterfaceIPv6,
		},

		{
			scenario: "ipv4 sockets in different namespaces can connect to one another",
			function: testLocalNetworkConnectNamespacesIPv4,
		},

		{
			scenario: "ipv6 sockets in different namespaces can connect to one another",
			function: testLocalNetworkConnectNamespacesIPv6,
		},

		{
			scenario: "local sockets can establish connections to foreign networks when a dial function is configured",
			function: testLocalNetworkOutboundConnect,
		},

		{
			scenario: "local sockets can receive inbound connections from foreign network when a listen function is configured",
			function: testLocalNetworkInboundAccept,
		},
	}

	ipv4, ipnet4, err := net.ParseCIDR("192.168.0.1/24")
	assert.OK(t, err)

	ipv6, ipnet6, err := net.ParseCIDR("fe80::1/64")
	assert.OK(t, err)

	ipnet4.IP = ipv4
	ipnet6.IP = ipv6

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			test.function(t,
				network.NewLocalNetwork(ipnet4, ipnet6),
			)
		})
	}
}

func testLocalNetworkInterfaces(t *testing.T, n *network.LocalNetwork) {
	ns, err := n.CreateNamespace(nil)
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
	assert.Equal(t, lo0Addrs[0].String(), "127.0.0.1/8")
	assert.Equal(t, lo0Addrs[1].String(), "::1/128")

	en0 := ifaces[1]
	assert.Equal(t, en0.Index(), 1)
	assert.Equal(t, en0.MTU(), 1500)
	assert.Equal(t, en0.Name(), "en0")
	assert.Equal(t, en0.Flags(), net.FlagUp)

	en0Addrs, err := en0.Addrs()
	assert.OK(t, err)
	assert.Equal(t, len(en0Addrs), 2)
	assert.Equal(t, en0Addrs[0].String(), "192.168.0.1/24")
	assert.Equal(t, en0Addrs[1].String(), "fe80::1/64")
}

func testLocalNetworkConnectLoopbackIPv4(t *testing.T, n *network.LocalNetwork) {
	testLocalNetworkConnect(t, n, &network.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
		Port: 80,
	})
}

func testLocalNetworkConnectLoopbackIPv6(t *testing.T, n *network.LocalNetwork) {
	testLocalNetworkConnect(t, n, &network.SockaddrInet6{
		Addr: [16]byte{15: 1},
		Port: 80,
	})
}

func testLocalNetworkConnectInterfaceIPv4(t *testing.T, n *network.LocalNetwork) {
	testLocalNetworkConnect(t, n, &network.SockaddrInet4{
		Addr: [4]byte{192, 168, 0, 1},
		Port: 80,
	})
}

func testLocalNetworkConnectInterfaceIPv6(t *testing.T, n *network.LocalNetwork) {
	testLocalNetworkConnect(t, n, &network.SockaddrInet6{
		Addr: [16]byte{0: 0xfe, 1: 0x80, 15: 1},
		Port: 80,
	})
}

func testLocalNetworkConnect(t *testing.T, n *network.LocalNetwork, bind network.Sockaddr) {
	ns, err := n.CreateNamespace(nil)
	assert.OK(t, err)
	testNamespaceConnect(t, ns, bind)
}

func testLocalNetworkConnectNamespacesIPv4(t *testing.T, n *network.LocalNetwork) {
	testLocalNetworkConnectNamespaces(t, n, network.INET)
}

func testLocalNetworkConnectNamespacesIPv6(t *testing.T, n *network.LocalNetwork) {
	testLocalNetworkConnectNamespaces(t, n, network.INET6)
}

func testLocalNetworkConnectNamespaces(t *testing.T, n *network.LocalNetwork, family network.Family) {
	ns1, err := n.CreateNamespace(nil)
	assert.OK(t, err)

	ns2, err := n.CreateNamespace(nil)
	assert.OK(t, err)

	ifaces1, err := ns1.Interfaces()
	assert.OK(t, err)
	assert.Equal(t, len(ifaces1), 2)

	addrs1, err := ifaces1[1].Addrs()
	assert.OK(t, err)

	server, err := ns1.Socket(family, network.STREAM, network.TCP)
	assert.OK(t, err)
	defer server.Close()

	assert.OK(t, server.Listen(1))
	serverAddr, err := server.Name()
	assert.OK(t, err)

	switch a := serverAddr.(type) {
	case *network.SockaddrInet4:
		for _, addr := range addrs1 {
			if ipnet := addr.(*net.IPNet); ipnet.IP.To4() != nil {
				copy(a.Addr[:], ipnet.IP.To4())
			}
		}
	case *network.SockaddrInet6:
		for _, addr := range addrs1 {
			if ipnet := addr.(*net.IPNet); ipnet.IP.To4() == nil {
				copy(a.Addr[:], ipnet.IP)
			}
		}
	}

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
	assert.Equal(t, network.SockaddrAddrPort(peer), network.SockaddrAddrPort(serverAddr))

	name, err := client.Name()
	assert.OK(t, err)
	assert.Equal(t, network.SockaddrAddrPort(name), network.SockaddrAddrPort(addr))

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

func testLocalNetworkOutboundConnect(t *testing.T, n *network.LocalNetwork) {
	ns1 := network.Host()

	ifaces1, err := ns1.Interfaces()
	assert.OK(t, err)
	assert.NotEqual(t, len(ifaces1), 0)

	var hostAddr *network.SockaddrInet4
	for _, iface := range ifaces1 {
		if hostAddr != nil {
			break
		}
		if (iface.Flags() & net.FlagUp) == 0 {
			continue
		}
		if (iface.Flags() & net.FlagLoopback) != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		assert.OK(t, err)

		for _, addr := range addrs {
			if a, ok := addr.(*net.IPNet); ok {
				if ipv4 := a.IP.To4(); ipv4 != nil {
					hostAddr = &network.SockaddrInet4{Addr: ([4]byte)(ipv4)}
					break
				}
			}
		}
	}
	assert.NotEqual(t, hostAddr, nil)

	server, err := ns1.Socket(network.INET, network.STREAM, network.TCP)
	assert.OK(t, err)
	defer server.Close()

	assert.OK(t, server.Bind(hostAddr))
	assert.OK(t, server.Listen(1))
	serverAddr, err := server.Name()
	assert.OK(t, err)

	var dialer net.Dialer
	ns2, err := n.CreateNamespace(nil, network.DialFunc(dialer.DialContext))
	assert.OK(t, err)

	client, err := ns2.Socket(network.INET, network.STREAM, network.TCP)
	assert.OK(t, err)
	defer client.Close()

	assert.Error(t, client.Connect(serverAddr), network.EINPROGRESS)
	assert.OK(t, waitReadyRead(server))

	conn, addr, err := server.Accept()
	assert.OK(t, err)
	defer conn.Close()
	assert.NotEqual(t, addr, nil)

	assert.OK(t, waitReadyWrite(client))
	peer, err := client.Peer()
	assert.OK(t, err)
	assert.Equal(t,
		network.SockaddrAddrPort(peer),
		network.SockaddrAddrPort(serverAddr))

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

func testLocalNetworkInboundAccept(t *testing.T, n *network.LocalNetwork) {
	ns, err := n.CreateNamespace(nil,
		network.ListenFunc(func(ctx context.Context, network, address string) (net.Listener, error) {
			_, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			return net.Listen(network, net.JoinHostPort("127.0.0.1", port))
		}),
	)
	assert.OK(t, err)

	sock, err := ns.Socket(network.INET, network.STREAM, network.TCP)
	assert.OK(t, err)
	defer sock.Close()

	assert.OK(t, sock.Listen(0))
	addr, err := sock.Name()
	assert.OK(t, err)

	addrPort := network.SockaddrAddrPort(addr)
	connAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(addrPort.Port())))
	conn, err := net.Dial("tcp", connAddr)
	assert.OK(t, err)
	defer conn.Close()

	assert.OK(t, waitReadyRead(sock))
	peer, peerAddr, err := sock.Accept()
	assert.OK(t, err)

	// verify that the address of the inbound connection matches the remote
	// address of the peer socket
	connLocalAddr := conn.LocalAddr().(*net.TCPAddr)
	peerAddrPort := network.SockaddrAddrPort(peerAddr)
	connAddrPort := netip.AddrPortFrom(netip.AddrFrom4(([4]byte)(connLocalAddr.IP)), uint16(connLocalAddr.Port))
	assert.Equal(t, peerAddrPort, connAddrPort)

	// verify that the inbound connection and the peer socket can exchange data
	size, err := conn.Write([]byte("message"))
	assert.OK(t, err)
	assert.Equal(t, size, 7)

	assert.OK(t, waitReadyRead(peer))
	buf := make([]byte, 32)
	size, _, _, err = peer.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, size, 7)
	assert.Equal(t, string(buf[:7]), "message")

	// exercise shutting down the write end of the inbound connection
	assert.OK(t, conn.(*net.TCPConn).CloseWrite())
	assert.OK(t, waitReadyRead(peer))

	size, _, _, err = peer.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, size, 0)

	// exercise shutting down the write end of the peer socket
	assert.OK(t, peer.Shutdown(network.SHUTWR))
	_, err = conn.Read(buf)
	assert.Equal(t, err, io.EOF)
}
