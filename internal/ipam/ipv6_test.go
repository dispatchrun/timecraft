package ipam_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/ipam"
)

func TestIPv6Pool(t *testing.T) {
	pool := ipam.NewIPv6Pool(ipam.IPv6{15: 1}, 120)
	name := pool.String()
	if name != "::1/120" {
		t.Errorf("wrong pool name: %q", name)
	}

	for i := 1; i < 256; i++ {
		ip, ok := pool.Get()
		if !ok {
			t.Fatalf("could not get address #%d", i)
		}
		if ip != (ipam.IPv6{15: byte(i)}) {
			t.Fatalf("wrong address at index %d: %s", i, ip)
		}
	}

	ip, ok := pool.Get()
	if ok {
		t.Fatalf("the pool should have been exhausted but it gave %s", ip)
	}

	for i := 50; i < 60; i++ {
		ip := ipam.IPv6{15: byte(i)}
		pool.Put(ip)
	}

	for i := 0; i < 10; i++ {
		ip, ok := pool.Get()
		if !ok {
			t.Fatalf("could not recycle address #%d", i)
		}
		if ip[15] != 50+byte(i) {
			t.Fatalf("wrong address recycled at index %d: %s", i, ip)
		}
	}
}

func BenchmarkIPv6Pool(b *testing.B) {
	pool := ipam.NewIPv6Pool(ipam.IPv6{15: 1}, 120)
	used := make([]ipam.IPv6, 0, 256)

	for i := 0; i < b.N; i++ {
		ip, ok := pool.Get()
		if !ok {
			for _, ip := range used {
				pool.Put(ip)
			}
			used = used[:0]
		} else {
			used = append(used, ip)
		}
	}
}
