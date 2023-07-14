package ipam

import (
	"encoding/binary"
	"fmt"
	"math/bits"
	"net"
	"net/netip"
)

type IPv6 [16]byte

func (ip IPv6) String() string {
	return ip.Addr().String()
}

func (ip IPv6) Addr() netip.Addr {
	return netip.AddrFrom16(ip)
}

func (ip IPv6) add(n int) IPv6 {
	hi := binary.BigEndian.Uint64(ip[:8])
	lo := binary.BigEndian.Uint64(ip[8:])

	x, c := bits.Add64(lo, uint64(n), 0)
	y, _ := bits.Add64(hi, 0, c)

	binary.BigEndian.PutUint64(ip[:8], y)
	binary.BigEndian.PutUint64(ip[8:], x)
	return ip
}

func (ip IPv6) sub(sub IPv6) int {
	hi1 := binary.BigEndian.Uint64(ip[:8])
	lo1 := binary.BigEndian.Uint64(ip[8:])

	hi2 := binary.BigEndian.Uint64(sub[:8])
	lo2 := binary.BigEndian.Uint64(sub[8:])

	_, c := bits.Sub64(hi1, hi2, 0)
	d, _ := bits.Sub64(lo1, lo2, c)
	return int(d)
}

func (ip IPv6) mask(n int) IPv6 {
	var hi, lo uint64
	if n < 64 {
		lo = 0
		hi = ^uint64(0) << uint(64-n)
	} else {
		lo = ^uint64(0) << uint(64-(n-64))
		hi = ^uint64(0)
	}
	binary.BigEndian.PutUint64(ip[:8], hi)
	binary.BigEndian.PutUint64(ip[8:], lo)
	return ip
}

func (ip IPv6) prefix(mask IPv6) IPv6 {
	hi1 := binary.LittleEndian.Uint64(ip[:8])
	lo1 := binary.LittleEndian.Uint64(ip[8:])

	hi2 := binary.LittleEndian.Uint64(mask[:8])
	lo2 := binary.LittleEndian.Uint64(mask[8:])

	binary.LittleEndian.PutUint64(ip[:8], hi1&hi2)
	binary.LittleEndian.PutUint64(ip[8:], lo1&lo2)
	return ip
}

type IPv6Pool struct {
	mask IPv6
	addr IPv6
	base IPv6
	bits bitset
}

func NewIPv6Pool(ip IPv6, nbits int) *IPv6Pool {
	p := new(IPv6Pool)
	p.Reset(ip, nbits)
	return p
}

func (p *IPv6Pool) String() string {
	hi := binary.BigEndian.Uint64(p.mask[:8])
	lo := binary.BigEndian.Uint64(p.mask[8:])
	n := bits.TrailingZeros64(hi)
	n += bits.TrailingZeros64(lo)
	return fmt.Sprintf("%s/%d", p.base, 128-n)
}

func (p *IPv6Pool) Reset(ip IPv6, nbits int) {
	p.mask = ip.mask(nbits)
	p.addr = ip.prefix(p.mask)
	p.base = ip
	p.bits.clear()
}

func (p *IPv6Pool) GetIP() net.IP {
	ip, ok := p.Get()
	if !ok {
		return nil
	}
	return ip[:]
}

func (p *IPv6Pool) Get() (IPv6, bool) {
	i := p.bits.findFirstZeroBit()
	a := p.base.add(i)

	if a.prefix(p.mask) != p.addr {
		return a, false
	}

	p.bits.grow(i + 1)
	p.bits.set(i)
	return p.base.add(i), true
}

func (p *IPv6Pool) PutIP(ip net.IP) {
	p.Put((IPv6)(ip))
}

func (p *IPv6Pool) Put(ip IPv6) {
	i := ip.sub(p.base)
	if !p.bits.has(i) {
		panic("BUG: unused IP address returned to pool")
	}
	p.bits.unset(i)
}
