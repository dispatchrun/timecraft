package timelog

import (
	"time"

	"github.com/stealthrocket/timecraft/pkg/format/logsegment"
	"github.com/stealthrocket/timecraft/pkg/format/types"
)

type Hash struct {
	Algorithm string
	Digest    string
}

type Compression uint32

const (
	Uncompressed Compression = Compression(types.CompressionUncompressed)
	Snappy       Compression = Compression(types.CompressionSnappy)
	Zstd         Compression = Compression(types.CompressionZstd)
)

type MemoryAccessType uint32

const (
	MemoryRead  MemoryAccessType = MemoryAccessType(logsegment.MemoryAccessTypeMemoryRead)
	MemoryWrite MemoryAccessType = MemoryAccessType(logsegment.MemoryAccessTypeMemoryWrite)
)

type MemoryAccess struct {
	Memory []byte
	Offset uint32
	Access MemoryAccessType
}

type Runtime struct {
	Runtime   string
	Version   string
	Functions []Function
}

type Function struct {
	Module string
	Name   string
}

type Process struct {
	ID               Hash
	Image            Hash
	StartTime        time.Time
	Args             []string
	Environ          []string
	ParentProcessID  Hash
	ParentForkOffset int64
}

type LogHeader struct {
	Runtime     Runtime
	Process     Process
	Segment     uint32
	Compression Compression
}

type Record struct {
	Timestamp    time.Time
	Function     int
	Params       []uint64
	Results      []uint64
	MemoryAccess []MemoryAccess
}

var (
	tl0 = []byte("TL.0")
	tl1 = []byte("TL.1")
	tl2 = []byte("TL.2")
	tl3 = []byte("TL.3")
)
