package ipam

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"net/netip"
)

type IPv4 [4]byte

func (ip IPv4) String() string {
	return ip.Addr().String()
}

func (ip IPv4) Addr() netip.Addr {
	return netip.AddrFrom4(ip)
}

func (ip IPv4) add(n int) IPv4 {
	u := binary.BigEndian.Uint32(ip[:])
	u += uint32(n)
	binary.BigEndian.PutUint32(ip[:], u)
	return ip
}

func (ip IPv4) sub(sub IPv4) int {
	u := binary.BigEndian.Uint32(ip[:])
	v := binary.BigEndian.Uint32(sub[:])
	return int(u - v)
}

func (ip IPv4) mask(n int) IPv4 {
	m := ^uint32(0) << uint(32-n)
	binary.BigEndian.PutUint32(ip[:], m)
	return ip
}

func (ip IPv4) prefix(mask IPv4) IPv4 {
	u := binary.LittleEndian.Uint32(ip[:])
	v := binary.LittleEndian.Uint32(mask[:])
	binary.LittleEndian.PutUint32(ip[:], u&v)
	return ip
}

type IPv4Pool struct {
	mask IPv4
	addr IPv4
	base IPv4
	bits bitset
}

func NewIPv4Pool(ip IPv4, nbits int) *IPv4Pool {
	p := new(IPv4Pool)
	p.Reset(ip, nbits)
	return p
}

func (p *IPv4Pool) String() string {
	u := binary.BigEndian.Uint32(p.mask[:])
	n := bits.TrailingZeros32(u)
	return fmt.Sprintf("%s/%d", p.base, 32-n)
}

func (p *IPv4Pool) Reset(ip IPv4, nbits int) {
	p.mask = ip.mask(nbits)
	p.addr = ip.prefix(p.mask)
	p.base = ip
	p.bits.clear()
}

func (p *IPv4Pool) Get() (IPv4, bool) {
	i := p.bits.findFirstZeroBit()
	a := p.base.add(i)

	if a.prefix(p.mask) != p.addr {
		return a, false
	}

	p.bits.grow(i + 1)
	p.bits.set(i)
	return p.base.add(i), true
}

func (p *IPv4Pool) Put(ip IPv4) {
	i := ip.sub(p.base)
	if !p.bits.has(i) {
		panic("BUG: unused IP address returned to pool")
	}
	p.bits.unset(i)
}
