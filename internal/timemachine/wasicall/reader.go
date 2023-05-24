package wasicall

import (
	"time"

	"github.com/stealthrocket/timecraft/internal/timemachine"
)

// Reader reads recorded system calls.
type Reader struct {
	reader timemachine.RecordReader
}

// ReadSyscall reads a recorded system call.
func (r *Reader) ReadSyscall() (time.Time, Syscall, error) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		return time.Time{}, nil, err
	}
	_ = record
	panic("not implemented")
}
