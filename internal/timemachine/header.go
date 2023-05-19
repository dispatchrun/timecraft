package timemachine

import "time"

// Header is a log header.
type Header struct {
	Runtime     Runtime
	Process     Process
	Segment     uint32
	Compression Compression
}

type Runtime struct {
	Runtime   string
	Version   string
	Functions []Function
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
