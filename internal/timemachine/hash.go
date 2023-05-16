package timemachine

import (
	"crypto/sha256"
	"encoding/hex"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/types"
)

type Hash struct {
	Algorithm, Digest string
}

func SHA256(b []byte) Hash {
	digest := sha256.Sum256(b)
	return Hash{Algorithm: "sha256", Digest: hex.EncodeToString(digest[:])}
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
