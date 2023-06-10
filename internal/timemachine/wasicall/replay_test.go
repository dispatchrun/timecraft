package wasicall

import (
	"context"
	"encoding/binary"
	"reflect"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

func TestReplay(t *testing.T) {
	for _, syscall := range validSyscalls {
		t.Run(syscallString(syscall), func(t *testing.T) {
			startTime := time.Now()
			var recordBytes []byte

			recorder := NewRecorder(&resultsSystem{syscall}, startTime, func(b *timemachine.RecordBuilder) {
				recordBytes = b.Bytes()
			})
			call(context.Background(), recorder, syscall)

			if recordBytes == nil {
				t.Fatalf("record was not recorded")
			}

			size, recordBytes := binary.LittleEndian.Uint32(recordBytes[:4]), recordBytes[4:]
			if size != uint32(len(recordBytes)) {
				t.Fatalf("record size prefix is missing or incorrect: got %d, expect %d", size, len(recordBytes))
			}

			record := timemachine.MakeRecord(recordBytes, startTime, 0)

			replay := NewReplay(stream.NewReader(record))

			// Call the replay system with the same params. It will panic if
			// the recorded syscall params differ.
			syscallWithResults := call(context.Background(), replay, syscall)

			// Now check that the syscall results match.
			if !reflect.DeepEqual(syscall, syscallWithResults) {
				t.Error("unexpected syscall results")
				t.Logf("actual: %#v", syscallWithResults.Results())
				t.Logf("expect: %#v", syscall.Results())
			}
		})
	}
}
