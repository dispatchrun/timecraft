package ipam_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/ipam"
)

func TestIPv4Pool(t *testing.T) {
	pool := ipam.NewIPv4Pool(ipam.IPv4{192, 168, 0, 1}, 24)
	name := pool.String()
	if name != "192.168.0.1/24" {
		t.Errorf("wrong pool name: %q", name)
	}

	for i := 1; i < 256; i++ {
		ip, ok := pool.Get()
		if !ok {
			t.Fatalf("could not get address #%d", i)
		}
		if ip != (ipam.IPv4{192, 168, 0, byte(i)}) {
			t.Fatalf("wrong address at index %d: %s", i, ip)
		}
	}

	ip, ok := pool.Get()
	if ok {
		t.Fatalf("the pool should have been exhausted but it gave %s", ip)
	}

	for i := 50; i < 60; i++ {
		ip := ipam.IPv4{192, 168, 0, byte(i)}
		pool.Put(ip)
	}

	for i := 0; i < 10; i++ {
		ip, ok := pool.Get()
		if !ok {
			t.Fatalf("could not recycle address #%d", i)
		}
		switch {
		case ip[0] != 192:
		case ip[1] != 168:
		case ip[2] != 0:
		case ip[3] != 50+byte(i):
		default:
			continue
		}
		t.Fatalf("wrong address recycled at index %d: %s", i, ip)
	}
}

func BenchmarkIPv4Pool(b *testing.B) {
	pool := ipam.NewIPv4Pool(ipam.IPv4{192, 168, 0, 1}, 24)
	used := make([]ipam.IPv4, 0, 256)

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
