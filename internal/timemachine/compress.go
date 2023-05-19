package timemachine

import (
	"fmt"
	"sync"

	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/stealthrocket/timecraft/format/types"
)

type Compression = types.Compression

const (
	Uncompressed Compression = types.CompressionUncompressed
	Snappy       Compression = types.CompressionSnappy
	Zstd         Compression = types.CompressionZstd
)

var (
	zstdEncoderPool objectPool[*zstd.Encoder]
	zstdDecoderPool objectPool[*zstd.Decoder]
)

type objectPool[T any] struct {
	pool sync.Pool
}

func (p *objectPool[T]) get(newObject func() T) T {
	v, ok := p.pool.Get().(T)
	if ok {
		return v
	}
	return newObject()
}

func (p *objectPool[T]) put(obj T) {
	p.pool.Put(obj)
}

func compress(dst, src []byte, compression Compression) []byte {
	switch compression {
	case Snappy:
		return snappy.Encode(dst, src)
	case Zstd:
		enc := zstdEncoderPool.get(func() *zstd.Encoder {
			e, _ := zstd.NewWriter(nil,
				zstd.WithEncoderCRC(false),
				zstd.WithEncoderConcurrency(1),
				zstd.WithEncoderLevel(zstd.SpeedFastest),
			)
			return e
		})
		defer zstdEncoderPool.put(enc)
		return enc.EncodeAll(src, dst[:0])
	default:
		return append(dst[:0], src...)
	}
}

func decompress(dst, src []byte, compression Compression) ([]byte, error) {
	switch compression {
	case Snappy:
		return snappy.Decode(dst, src)
	case Zstd:
		dec := zstdDecoderPool.get(func() *zstd.Decoder {
			d, _ := zstd.NewReader(nil,
				zstd.IgnoreChecksum(true),
				zstd.WithDecoderConcurrency(1),
			)
			return d
		})
		defer zstdDecoderPool.put(dec)
		return dec.DecodeAll(src, dst[:0])
	default:
		return dst, fmt.Errorf("unknown compression format: %d", compression)
	}
}
