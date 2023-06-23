package sandbox

type ringbuf[T any] struct {
	buf []T
	off int32
	end int32
}

func makeRingBuffer[T any](n int) ringbuf[T] {
	return ringbuf[T]{buf: make([]T, n)}
}

func (rb *ringbuf[T]) len() int {
	return int(rb.end - rb.off)
}

func (rb *ringbuf[T]) cap() int {
	return len(rb.buf)
}

func (rb *ringbuf[T]) avail() int {
	return int(rb.off) + (len(rb.buf) - int(rb.end))
}

func (rb *ringbuf[T]) index(i int) *T {
	return &rb.buf[int(rb.off)+i]
}

func (rb *ringbuf[T]) discard(n int) {
	if n < 0 {
		panic("BUG: discard negative count")
	}
	if n > rb.len() {
		panic("BUG: discard more values than exist in the buffer")
	}
	if rb.off += int32(n); rb.off == rb.end {
		rb.off = 0
		rb.end = 0
	}
}

func (rb *ringbuf[T]) peek(values []T, off int) int {
	return copy(values, rb.buf[rb.off+int32(off):rb.end])
}

func (rb *ringbuf[T]) read(values []T) int {
	n := rb.peek(values, 0)
	rb.discard(n)
	return n
}

func (rb *ringbuf[T]) write(values []T) int {
	if (len(rb.buf) - int(rb.end)) < len(values) {
		rb.pack()
	}
	n := copy(rb.buf[rb.end:], values)
	rb.end += int32(n)
	return n
}

func (rb *ringbuf[T]) append(values ...T) {
	if (len(rb.buf) - int(rb.end)) < len(values) {
		rb.pack()
	}
	rb.buf = append(rb.buf[:rb.end], values...)
	rb.buf = rb.buf[:cap(rb.buf)]
	rb.end += int32(len(values))
}

func (rb *ringbuf[T]) pack() {
	if rb.off > 0 {
		n := copy(rb.buf, rb.buf[rb.off:rb.end])
		rb.end = int32(n)
		rb.off = 0
	}
}
