package timemachine

import "sync"

const defaultBufferSize = 4096

type buffer struct{ data []byte }

type bufferPool struct{ pool sync.Pool }

func (p *bufferPool) get(size int) *buffer {
	b, _ := p.pool.Get().(*buffer)
	if b != nil {
		if size <= cap(b.data) {
			b.data = b.data[:size]
			return b
		}
		p.put(b)
		b = nil
	}
	return newBuffer(size)
}

func (p *bufferPool) put(b *buffer) {
	if b != nil {
		p.pool.Put(b)
	}
}

var (
	frameBufferPool        bufferPool
	compressedBufferPool   bufferPool
	uncompressedBufferPool bufferPool
)

func newBuffer(size int) *buffer {
	return &buffer{data: make([]byte, size, align(size))}
}

func releaseBuffer(buf **buffer, pool *bufferPool) {
	if b := *buf; b != nil {
		*buf = nil
		pool.put(b)
	}
}

func align(size int) int {
	return ((size + (defaultBufferSize - 1)) / defaultBufferSize) * defaultBufferSize
}
