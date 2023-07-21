package sandbox_test

import (
	"context"
	"io"
	"net"
	"net/netip"
	"strconv"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

func TestLocalNetwork(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T, *sandbox.LocalNetwork)
	}{
		{
			scenario: "a local network namespace has two interfaces",
			function: testLocalNetworkInterfaces,
		},

		{
			scenario: "ipv4 stream sockets can connect to one another on a loopback interface",
			function: testLocalNetworkConnectStreamLoopbackIPv4,
		},

		{
			scenario: "ipv6 stream sockets can connect to one another on a loopback interface",
			function: testLocalNetworkConnectStreamLoopbackIPv6,
		},

		{
			scenario: "ipv4 stream sockets can connect to one another on a network interface",
			function: testLocalNetworkConnectStreamInterfaceIPv4,
		},

		{
			scenario: "ipv6 stream sockets can connect to one another on a network interface",
			function: testLocalNetworkConnectStreamInterfaceIPv6,
		},

		{
			scenario: "ipv4 stream sockets in different namespaces can connect to one another",
			function: testLocalNetworkConnectStreamNamespacesIPv4,
		},

		{
			scenario: "ipv6 stream sockets in different namespaces can connect to one another",
			function: testLocalNetworkConnectStreamNamespacesIPv6,
		},

		{
			scenario: "stream sockets can establish connections to foreign networks when a dial function is configured",
			function: testLocalNetworkOutboundConnectStream,
		},

		{
			scenario: "stream sockets can receive inbound connections from foreign network when a listen function is configured",
			function: testLocalNetworkInboundAccept,
		},

		{
			scenario: "ipv4 datagram sockets can connect to one another on the loopback interface",
			function: testLocalNetworkConnectDatagramIPv4,
		},

		{
			scenario: "ipv6 datagram sockets can connect to one another on the loopback interface",
			function: testLocalNetworkConnectDatagramIPv6,
		},

		{
			scenario: "ipv4 datagram sockets can exchange datagrams without being connected to one another",
			function: testLocalNetworkExchangeDatagramIPv4,
		},

		{
			scenario: "ipv6 datagram sockets can exchange datagrams without being connected to one another",
			function: testLocalNetworkExchangeDatagramIPv6,
		},

		{
			scenario: "datagram sockets can receive messages from foreign networks when a listen packet function is configured",
			function: testLocalNetworkInboundDatagram,
		},

		{
			scenario: "datagram sockets can send messages to foreign networks when a listen packet function is configured",
			function: testLocalNetworkOutboundDatagram,
		},
	}

	ipnet4, err := netip.ParsePrefix("192.168.0.1/24")
	assert.OK(t, err)

	ipnet6, err := netip.ParsePrefix("fe80::1/64")
	assert.OK(t, err)

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			test.function(t,
				sandbox.NewLocalNetwork(ipnet4, ipnet6),
			)
		})
	}
}

func testLocalNetworkInterfaces(t *testing.T, n *sandbox.LocalNetwork) {
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

func testLocalNetworkConnectStreamLoopbackIPv4(t *testing.T, n *sandbox.LocalNetwork) {
	testLocalNetworkConnectStream(t, n, &sandbox.SockaddrInet4{
		Addr: [4]byte{127, 0, 0, 1},
		Port: 80,
	})
}

func testLocalNetworkConnectStreamLoopbackIPv6(t *testing.T, n *sandbox.LocalNetwork) {
	testLocalNetworkConnectStream(t, n, &sandbox.SockaddrInet6{
		Addr: [16]byte{15: 1},
		Port: 80,
	})
}

func testLocalNetworkConnectStreamInterfaceIPv4(t *testing.T, n *sandbox.LocalNetwork) {
	testLocalNetworkConnectStream(t, n, &sandbox.SockaddrInet4{
		Addr: [4]byte{192, 168, 0, 1},
		Port: 80,
	})
}

func testLocalNetworkConnectStreamInterfaceIPv6(t *testing.T, n *sandbox.LocalNetwork) {
	testLocalNetworkConnectStream(t, n, &sandbox.SockaddrInet6{
		Addr: [16]byte{0: 0xfe, 1: 0x80, 15: 1},
		Port: 80,
	})
}

func testLocalNetworkConnectStream(t *testing.T, n *sandbox.LocalNetwork, bind sandbox.Sockaddr) {
	ns, err := n.CreateNamespace(nil)
	assert.OK(t, err)
	testNamespaceConnectStream(t, ns, bind)
}

func testLocalNetworkConnectStreamNamespacesIPv4(t *testing.T, n *sandbox.LocalNetwork) {
	testLocalNetworkConnectStreamNamespaces(t, n, sandbox.INET)
}

func testLocalNetworkConnectStreamNamespacesIPv6(t *testing.T, n *sandbox.LocalNetwork) {
	testLocalNetworkConnectStreamNamespaces(t, n, sandbox.INET6)
}

func testLocalNetworkConnectStreamNamespaces(t *testing.T, n *sandbox.LocalNetwork, family sandbox.Family) {
	ns1, err := n.CreateNamespace(nil)
	assert.OK(t, err)

	ns2, err := n.CreateNamespace(nil)
	assert.OK(t, err)

	ifaces1, err := ns1.Interfaces()
	assert.OK(t, err)
	assert.Equal(t, len(ifaces1), 2)

	addrs1, err := ifaces1[1].Addrs()
	assert.OK(t, err)

	server, err := ns1.Socket(family, sandbox.STREAM, sandbox.TCP)
	assert.OK(t, err)
	defer server.Close()
	server.SetNonblock(true)

	assert.OK(t, server.Listen(1))
	serverAddr, err := server.Name()
	assert.OK(t, err)

	switch a := serverAddr.(type) {
	case *sandbox.SockaddrInet4:
		for _, addr := range addrs1 {
			if ipnet := addr.(*net.IPNet); ipnet.IP.To4() != nil {
				copy(a.Addr[:], ipnet.IP.To4())
			}
		}
	case *sandbox.SockaddrInet6:
		for _, addr := range addrs1 {
			if ipnet := addr.(*net.IPNet); ipnet.IP.To4() == nil {
				copy(a.Addr[:], ipnet.IP)
			}
		}
	}

	client, err := ns2.Socket(family, sandbox.STREAM, sandbox.TCP)
	assert.OK(t, err)
	defer client.Close()
	client.SetNonblock(true)

	assert.Error(t, client.Connect(serverAddr), sandbox.EINPROGRESS)
	assert.OK(t, waitReadyRead(server))

	conn, addr, err := server.Accept()
	assert.OK(t, err)
	defer conn.Close()
	conn.SetNonblock(true)

	assert.OK(t, waitReadyWrite(client))
	peer, err := client.Peer()
	assert.OK(t, err)
	assert.Equal(t, sandbox.SockaddrAddrPort(peer), sandbox.SockaddrAddrPort(serverAddr))

	name, err := client.Name()
	assert.OK(t, err)
	assert.Equal(t, sandbox.SockaddrAddrPort(name), sandbox.SockaddrAddrPort(addr))

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

func testLocalNetworkOutboundConnectStream(t *testing.T, n *sandbox.LocalNetwork) {
	ns1 := sandbox.Host()

	ifaces1, err := ns1.Interfaces()
	assert.OK(t, err)
	assert.NotEqual(t, len(ifaces1), 0)

	var hostAddr *sandbox.SockaddrInet4
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
					hostAddr = &sandbox.SockaddrInet4{Addr: ([4]byte)(ipv4)}
					break
				}
			}
		}
	}
	assert.NotEqual(t, hostAddr, nil)

	server, err := ns1.Socket(sandbox.INET, sandbox.STREAM, sandbox.TCP)
	assert.OK(t, err)
	defer server.Close()
	server.SetNonblock(true)

	assert.OK(t, server.Bind(hostAddr))
	assert.OK(t, server.Listen(1))
	serverAddr, err := server.Name()
	assert.OK(t, err)

	var dialer net.Dialer
	ns2, err := n.CreateNamespace(nil, sandbox.DialFunc(dialer.DialContext))
	assert.OK(t, err)

	client, err := ns2.Socket(sandbox.INET, sandbox.STREAM, sandbox.TCP)
	assert.OK(t, err)
	defer client.Close()
	client.SetNonblock(true)

	assert.Error(t, client.Connect(serverAddr), sandbox.EINPROGRESS)
	assert.OK(t, waitReadyRead(server))

	conn, addr, err := server.Accept()
	assert.OK(t, err)
	defer conn.Close()
	conn.SetNonblock(true)
	assert.NotEqual(t, addr, nil)

	assert.OK(t, waitReadyWrite(client))
	peer, err := client.Peer()
	assert.OK(t, err)
	assert.Equal(t,
		sandbox.SockaddrAddrPort(peer),
		sandbox.SockaddrAddrPort(serverAddr))

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

func testLocalNetworkInboundAccept(t *testing.T, n *sandbox.LocalNetwork) {
	ns, err := n.CreateNamespace(nil,
		sandbox.ListenFunc(func(ctx context.Context, network, address string) (net.Listener, error) {
			_, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			return net.Listen(network, net.JoinHostPort("127.0.0.1", port))
		}),
	)
	assert.OK(t, err)

	sock, err := ns.Socket(sandbox.INET, sandbox.STREAM, sandbox.TCP)
	assert.OK(t, err)
	defer sock.Close()

	assert.OK(t, sock.Listen(0))
	addr, err := sock.Name()
	assert.OK(t, err)

	addrPort := sandbox.SockaddrAddrPort(addr)
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
	peerAddrPort := sandbox.SockaddrAddrPort(peerAddr)
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
	assert.OK(t, peer.Shutdown(sandbox.SHUTWR))
	_, err = conn.Read(buf)
	assert.Equal(t, err, io.EOF)
}

func testLocalNetworkConnectDatagramIPv4(t *testing.T, n *sandbox.LocalNetwork) {
	ns, err := n.CreateNamespace(nil)
	assert.OK(t, err)
	testNamespaceConnectDatagram(t, ns, &sandbox.SockaddrInet4{
		Addr: [4]byte{192, 168, 0, 1},
	})
}

func testLocalNetworkConnectDatagramIPv6(t *testing.T, n *sandbox.LocalNetwork) {
	ns, err := n.CreateNamespace(nil)
	assert.OK(t, err)
	testNamespaceConnectDatagram(t, ns, &sandbox.SockaddrInet6{
		Addr: [16]byte{0: 0xfe, 1: 0x80, 15: 1},
	})
}

func testLocalNetworkExchangeDatagramIPv4(t *testing.T, n *sandbox.LocalNetwork) {
	ns, err := n.CreateNamespace(nil)
	assert.OK(t, err)
	testNamespaceExchangeDatagram(t, ns, &sandbox.SockaddrInet4{
		Addr: [4]byte{192, 168, 0, 1},
	})
}

func testLocalNetworkExchangeDatagramIPv6(t *testing.T, n *sandbox.LocalNetwork) {
	ns, err := n.CreateNamespace(nil)
	assert.OK(t, err)
	testNamespaceExchangeDatagram(t, ns, &sandbox.SockaddrInet6{
		Addr: [16]byte{0: 0xfe, 1: 0x80, 15: 1},
	})
}

func testLocalNetworkInboundDatagram(t *testing.T, n *sandbox.LocalNetwork) {
	ns, err := n.CreateNamespace(nil,
		sandbox.ListenPacketFunc(func(ctx context.Context, network, address string) (net.PacketConn, error) {
			_, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			return net.ListenPacket(network, net.JoinHostPort("127.0.0.1", port))
		}),
	)
	assert.OK(t, err)

	sock, err := ns.Socket(sandbox.INET, sandbox.DGRAM, sandbox.UDP)
	assert.OK(t, err)
	defer sock.Close()

	assert.OK(t, sock.Bind(&sandbox.SockaddrInet4{}))
	addr, err := sock.Name()
	assert.OK(t, err)

	addrPort := sandbox.SockaddrAddrPort(addr)
	connAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(int(addrPort.Port())))
	conn, err := net.Dial("udp", connAddr)
	assert.OK(t, err)
	defer conn.Close()
	localAddr := conn.LocalAddr().(*net.UDPAddr)

	size, err := conn.Write([]byte("message"))
	assert.OK(t, err)
	assert.Equal(t, size, 7)

	assert.OK(t, waitReadyRead(sock))
	buf := make([]byte, 32)
	size, _, peer, err := sock.RecvFrom([][]byte{buf}, 0)
	assert.OK(t, err)
	assert.Equal(t, size, 7)
	assert.Equal(t, string(buf[:7]), "message")
	assert.Equal(t, sandbox.SockaddrAddrPort(peer), localAddr.AddrPort())
}

func testLocalNetworkOutboundDatagram(t *testing.T, n *sandbox.LocalNetwork) {
	hostAddrs, err := findNonLoopbackIPv4HostAddress()
	assert.OK(t, err)

	ns, err := n.CreateNamespace(nil,
		sandbox.ListenPacketFunc(func(ctx context.Context, network, address string) (net.PacketConn, error) {
			_, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			return net.ListenPacket(network, net.JoinHostPort("", port))
		}),
	)
	assert.OK(t, err)

	conn, err := net.ListenPacket("udp4", net.JoinHostPort(hostAddrs[0].String(), "0"))
	assert.OK(t, err)
	defer conn.Close()

	sock, err := ns.Socket(sandbox.INET, sandbox.DGRAM, sandbox.UDP)
	assert.OK(t, err)
	defer sock.Close()

	connAddr := conn.LocalAddr().(*net.UDPAddr)
	addrPort := connAddr.AddrPort()
	sendAddr := &sandbox.SockaddrInet4{
		Addr: addrPort.Addr().As4(),
		Port: int(addrPort.Port()),
	}

	size, err := sock.SendTo([][]byte{[]byte("message")}, sendAddr, 0)
	assert.OK(t, err)
	assert.Equal(t, size, 7)

	addr, err := sock.Name()
	assert.OK(t, err)

	buf := make([]byte, 32)
	size, peer, err := conn.ReadFrom(buf)
	assert.OK(t, err)
	assert.Equal(t, size, 7)
	assert.Equal(t, peer.(*net.UDPAddr).AddrPort().Port(), sandbox.SockaddrAddrPort(addr).Port())
}
