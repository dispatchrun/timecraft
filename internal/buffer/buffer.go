package buffer

import "sync"

const DefaultSize = 4096

type Buffer struct{ Data []byte }

func (buf *Buffer) Size() int64 {
	return int64(len(buf.Data))
}

type Pool struct{ pool sync.Pool }

func (p *Pool) Get(size int64) *Buffer {
	b, _ := p.pool.Get().(*Buffer)
	if b != nil {
		if int(size) <= cap(b.Data) {
			b.Data = b.Data[:size]
			return b
		}
		p.Put(b)
		b = nil
	}
	return New(size)
}

func (p *Pool) Put(b *Buffer) {
	if b != nil {
		p.pool.Put(b)
	}
}

func New(size int64) *Buffer {
	return &Buffer{Data: make([]byte, size, Align(size, DefaultSize))}
}

func Release(buf **Buffer, pool *Pool) {
	if b := *buf; b != nil {
		*buf = nil
		pool.Put(b)
	}
}

func Align(size, to int64) int64 {
	return ((size + (to - 1)) / to) * to
}
