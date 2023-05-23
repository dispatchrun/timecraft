package timemachine

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"

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

func UUIDv4(r io.Reader) Hash {
	var uuid [16]byte
	if _, err := io.ReadFull(r, uuid[:]); err != nil {
		panic("readfull")
	}
	uuid[6] = (uuid[6] & 0x0f) | 0x40 // version 4
	uuid[8] = (uuid[8] & 0x3f) | 0x80 // variant 1
	s := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", uuid[:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
	return Hash{Algorithm: "uuidv4", Digest: s}
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
