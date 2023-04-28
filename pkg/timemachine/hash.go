package timemachine

import (
	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/pkg/format/types"
)

type Hash struct {
	Algorithm, Digest string
}

func makeHash(h *types.Hash) Hash {
	return Hash{
		Algorithm: string(h.Algorithm()),
		Digest:    string(h.Digest()),
	}
}

func (h *Hash) prepend(b *flatbuffers.Builder) flatbuffers.UOffsetT {
	algorithm := b.CreateSharedString(h.Algorithm)
	digest := b.CreateString(h.Digest)
	types.HashStart(b)
	types.HashAddAlgorithm(b, algorithm)
	types.HashAddDigest(b, digest)
	return types.HashEnd(b)
}
