package timemachine

import (
	"fmt"
	"io"

	"github.com/stealthrocket/timecraft/format"
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
