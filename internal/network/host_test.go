package network_test

import (
	"net"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/network"
)

func TestHostNetwork(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T, network.Namespace)
	}{
		{
			scenario: "a host network namespace has at least one loopback interface",
			function: testHostNetworkInterface,
		},

		{
			scenario: "ipv4 stream sockets can connect to one another on the loopback interface",
			function: testNamespaceConnectStreamLoopbackIPv4,
		},

		{
			scenario: "ipv6 stream sockets can connect to one another on the loopback interface",
			function: testNamespaceConnectStreamLoopbackIPv6,
		},

		{
			scenario: "ipv4 datagram sockets can connect to one another on the loopback interface",
			function: testNamespaceConnectDatagramLoopbackIPv4,
		},

		{
			scenario: "ipv6 datagram sockets can connect to one another on the loopback interface",
			function: testNamespaceConnectDatagramLoopbackIPv6,
		},

		{
			scenario: "ipv4 sockets can exchange datagrams on the loopback interface",
			function: testNamespaceExchangeDatagramLoopbackIPv4,
		},

		{
			scenario: "ipv6 sockets can exchange datagrams on the loopback interface",
			function: testNamespaceExchangeDatagramLoopbackIPv6,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			test.function(t, network.Host())
		})
	}
}

func testHostNetworkInterface(t *testing.T, ns network.Namespace) {
	ifaces, err := ns.Interfaces()
	assert.OK(t, err)

	for _, iface := range ifaces {
		if (iface.Flags() & net.FlagLoopback) == 0 {
			continue
		}
		if (iface.Flags() & net.FlagUp) == 0 {
			continue
		}

		lo0 := iface
		assert.NotEqual(t, lo0.Name(), "")

		lo0Addrs, err := lo0.Addrs()
		assert.OK(t, err)

		ipv4 := false
		ipv6 := false
		for _, addr := range lo0Addrs {
			switch addr.String() {
			case "127.0.0.1/8":
				ipv4 = true
			case "::1/128":
				ipv6 = true
			}
		}
		assert.True(t, ipv4 && ipv6)
		return
	}

	t.Fatal("host network has not loopback interface")
}
