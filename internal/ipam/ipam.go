// Package ipam contains types to implement IP address management on IPv4 and
// IPv6 networks.
package ipam

import "net/netip"

// Pool is an interface implemented by the IPv4Pool and IPv6Pool types to
// abstract the type of IP addresses that are managed by the pool.
type Pool interface {
	// Obtains the next IP address, or returns nil if the pool was exhausted.
	GetAddr() (netip.Addr, bool)
	// Returns an IP address to the pool. The ip address must have been obtained
	// by a previous call to GetIP or the method panics.
	PutAddr(netip.Addr)
}

// NewPool constructs a pool of IP addresses for the network passed as argument.
func NewPool(prefix netip.Prefix) Pool {
	addr := prefix.Addr()
	bits := prefix.Bits()
	if addr.Is4() {
		return NewIPv4Pool(addr.As4(), bits)
	} else {
		return NewIPv6Pool(addr.As16(), bits)
	}
}
