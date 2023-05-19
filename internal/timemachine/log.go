package timemachine

import "time"

type RecordFIXME struct {
	Timestamp    time.Time
	Function     int
	Params       []uint64
	Results      []uint64
	MemoryAccess []MemoryAccess

	// Stack is buffer space for params/results.
	Stack [10]uint64
}
