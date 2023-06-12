package wasicall

import (
	"context"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

func TestRecord(t *testing.T) {
	for _, syscall := range syscalls {
		t.Run(syscallString(syscall), func(t *testing.T) {
			var recordID SyscallID
			var recordBytes []byte

			recorder := NewRecorder(&resultsSystem{syscall}, func(id SyscallID, b []byte) {
				recordID = id
				recordBytes = b
			})
			call(context.Background(), recorder, syscall)

			if recordBytes == nil {
				t.Fatalf("write was not recorded")
			} else if recordID != syscall.ID() {
				t.Fatalf("unexpected recorded syscall ID: got %v, expect %v", recordID, syscall.ID())
			}

			record := timemachine.Record{FunctionID: int(recordID), FunctionCall: recordBytes}

			reader := NewReader(stream.NewReader(record))
			_, recordSyscall, err := reader.ReadSyscall()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(syscall, recordSyscall) {
				t.Errorf("invalid write")
				t.Log("Actual:")
				spew.Dump(recordSyscall)
				t.Log("Expect:")
				spew.Dump(syscall)
			}
		})
	}
}
