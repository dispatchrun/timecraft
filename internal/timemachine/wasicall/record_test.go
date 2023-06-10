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

func TestRecord(t *testing.T) {
	for _, syscall := range syscalls {
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

			reader := NewReader(stream.NewReader(record))
			_, recordSyscall, err := reader.ReadSyscall()
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(syscall, recordSyscall) {
				t.Errorf("invalid record")
				t.Log("actual:", syscallString(recordSyscall))
				t.Log("expect:", syscallString(syscall))
			}
		})
	}
}
