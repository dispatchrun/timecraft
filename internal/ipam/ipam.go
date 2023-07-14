// Package ipam contains types to implement IP address management on IPv4 and
// IPv6 networks.
package ipam

import "net"

// Pool is an interface implemented by the IPv4Pool and IPv6Pool types to
// abstract the type of IP addresses that are managed by the pool.
type Pool interface {
	// Obtains the next IP address, or returns nil if the pool was exhausted.
	GetIP() net.IP
	// Returns an IP address to the pool. The ip address must have been obtained
	// by a previous call to GetIP or the method panics.
	PutIP(net.IP)
}

// NewPool constructs a pool of IP addresses for the network passed as argument.
func NewPool(ipnet *net.IPNet) Pool {
	ones, _ := ipnet.Mask.Size()
	if ipv4 := ipnet.IP.To4(); ipv4 != nil {
		return NewIPv4Pool((IPv4)(ipv4), ones)
	} else {
		return NewIPv6Pool((IPv6)(ipnet.IP), ones)
	}
}
