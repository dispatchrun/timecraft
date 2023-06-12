package wasicall

import (
	"context"
	"reflect"
	"testing"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go"
)

func TestReplay(t *testing.T) {
	for _, syscall := range syscalls {
		t.Run(syscallString(syscall), func(t *testing.T) {
			testReplay(t, syscall, nil)
		})
	}
}

func testReplay(t *testing.T, syscall Syscall, wrapSystem func(s wasi.System) wasi.System) {
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

	var replay wasi.System = NewReplay(stream.NewReader(record))

	if wrapSystem != nil {
		replay = wrapSystem(replay)
	}

	// Call the replay system with the same params. It will panic if
	// the recorded syscall params differ.
	syscallWithResults := call(context.Background(), replay, syscall)

	// Now check that the syscall results match.
	if !reflect.DeepEqual(syscall, syscallWithResults) {
		t.Error("unexpected syscall results")
		t.Logf("actual: %#v", syscallWithResults.Results())
		t.Logf("expect: %#v", syscall.Results())
	}
}
