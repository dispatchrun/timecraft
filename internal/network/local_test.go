package network_test

import (
	"net"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/network"
)

func TestLocalNetwork(t *testing.T) {
	tests := []struct {
		scenario string
		function func(*testing.T, network.Namespace)
	}{
		{
			scenario: "a local network namespace has one interface",
			function: testLocalNetworkInterface,
		},

		{
			scenario: "ipv4 sockets can connect to one another on the loopback interface",
			function: testNamespaceConnectLoopbackIPv4,
		},

		{
			scenario: "ipv6 sockets can connect to one another on the loopback interface",
			function: testNamespaceConnectLoopbackIPv6,
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
			test.function(t,
				network.NewLocalNamespace(nil),
			)
		})
	}
}

func testLocalNetworkInterface(t *testing.T, ns network.Namespace) {
	ifaces, err := ns.Interfaces()
	assert.OK(t, err)
	assert.Equal(t, len(ifaces), 1)

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
}
