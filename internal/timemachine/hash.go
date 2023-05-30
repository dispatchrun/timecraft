package timemachine

import (
	"encoding/binary"

	"github.com/stealthrocket/timecraft/format"
)

func HashProfile(processID format.UUID, profileType string, timeRange TimeRange) format.Hash {
	b := make([]byte, 32, 64)
	copy(b, processID[:])
	binary.LittleEndian.PutUint64(b[16:], uint64(timeRange.Start.UnixNano()))
	binary.LittleEndian.PutUint64(b[24:], uint64(timeRange.End.UnixNano()))
	b = append(b, profileType...)
	return format.SHA256(b)
}
