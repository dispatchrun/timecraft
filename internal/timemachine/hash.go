package timemachine

import (
	"fmt"
	"io"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/format/types"
)

type Hash = format.Hash

func SHA256(b []byte) Hash { return format.SHA256(b) }

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

func prependHash(b *flatbuffers.Builder, h Hash) flatbuffers.UOffsetT {
	algorithm := b.CreateSharedString(h.Algorithm)
	digest := b.CreateString(h.Digest)
	types.HashStart(b)
	types.HashAddAlgorithm(b, algorithm)
	types.HashAddDigest(b, digest)
	return types.HashEnd(b)
}
