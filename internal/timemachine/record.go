package timemachine

import (
	"time"

	"github.com/stealthrocket/timecraft/format/logsegment"
)

// Record is a read-only record from the log.
type Record struct {
	header       *Header
	record       logsegment.Record
	FunctionCall FunctionCall
}

// Timestamp is the record timestamp.
func (r *Record) Timestamp() time.Time {
	return r.header.Process.StartTime.Add(time.Duration(r.record.Timestamp()))
}

// FunctionID is the record's associated function ID.
func (r *Record) FunctionID() int {
	return int(r.record.FunctionId())
}
