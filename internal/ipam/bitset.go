package ipam

import "math/bits"

type bitset struct {
	bits []uint64
}

func (b *bitset) clear() {
	for i := range b.bits {
		b.bits[i] = 0
	}
}

func (b *bitset) grow(n int) {
	if n = (n + 63) / 64; n > len(b.bits) {
		bits := make([]uint64, n)
		copy(bits, b.bits)
		b.bits = bits
	}
}

func (b *bitset) has(i int) bool {
	index := uint(i) / 64
	shift := uint(i) % 64
	return (b.bits[index] & uint64(1<<shift)) != 0
}

func (b *bitset) set(i int) {
	index := uint(i) / 64
	shift := uint(i) % 64
	b.bits[index] |= 1 << shift
}

func (b *bitset) unset(i int) {
	index := uint(i) / 64
	shift := uint(i) % 64
	b.bits[index] &= ^uint64(1 << shift)
}

func (b *bitset) findFirstZeroBit() int {
	for i, v := range b.bits {
		if v != ^uint64(0) {
			return 64*i + bits.TrailingZeros64(^v)
		}
	}
	return 64 * len(b.bits)
}
