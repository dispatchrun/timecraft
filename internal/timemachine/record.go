package timemachine

import (
	"fmt"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
	"github.com/stealthrocket/timecraft/internal/buffer"
)

// Record is a read-only record from the log.
type Record struct {
	startTime time.Time
	functions []Function
	record    logsegment.Record
}

// MakeRecord creates a record from a buffer.
//
// The buffer must live as long as the record.
func MakeRecord(startTime time.Time, functions []Function, buffer []byte) (r Record) {
	r.Reset(startTime, functions, buffer)
	return
}

// Reset resets a record.
func (r *Record) Reset(startTime time.Time, functions []Function, buffer []byte) {
	r.startTime = startTime
	r.functions = functions
	r.record = *logsegment.GetSizePrefixedRootAsRecord(buffer, 0)
}

// Timestamp is the record timestamp.
func (r *Record) Timestamp() time.Time {
	return r.startTime.Add(time.Duration(r.record.Timestamp()))
}

// FunctionID is the record's associated function ID.
func (r *Record) FunctionID() int {
	return int(r.record.FunctionId())
}

// Function is the record's associated function.
func (r *Record) Function() (Function, error) {
	id := r.FunctionID()
	if id >= len(r.functions) {
		return Function{}, fmt.Errorf("invalid function %d", id)
	}
	return r.functions[id], nil
}

// FunctionCall returns the function call details.
func (r *Record) FunctionCall() []byte {
	return r.record.FunctionCallBytes()
}

// RecordBuilder is a builder for records.
type RecordBuilder struct {
	startTime    time.Time
	builder      *flatbuffers.Builder
	timestamp    int64
	functionID   uint32
	functionCall []byte
	finished     bool
}

// Reset resets the builder.
func (b *RecordBuilder) Reset(startTime time.Time) {
	b.startTime = startTime
	if b.builder == nil {
		b.builder = flatbuffers.NewBuilder(buffer.DefaultSize)
	} else {
		b.builder.Reset()
	}
	b.timestamp = 0
	b.functionID = 0
	b.functionCall = nil
	b.finished = false
}

// SetTimestamp sets the timestamp.
func (b *RecordBuilder) SetTimestamp(t time.Time) {
	if b.finished {
		panic("builder must be reset before timestamp can be set")
	}
	b.timestamp = int64(t.Sub(b.startTime))
}

// SetFunctionID sets the function ID.
func (b *RecordBuilder) SetFunctionID(id int) {
	if b.finished {
		panic("builder must be reset before function ID can be set")
	}
	b.functionID = uint32(id)
}

// SetFunctionCall sets the function call.
//
// The provided slice is retained until Bytes() is called and the record is
// serialized.
func (b *RecordBuilder) SetFunctionCall(functionCall []byte) {
	if b.finished {
		panic("builder must be reset before function call can be set")
	}
	b.functionCall = functionCall
}

// Bytes returns the serialized representation of the record.
func (b *RecordBuilder) Bytes() []byte {
	if !b.finished {
		b.build()
		b.finished = true
	}
	return b.builder.FinishedBytes()
}

func (b *RecordBuilder) build() {
	if b.builder == nil {
		panic("builder is not initialized")
	}
	b.buildWith(b.builder)
}

func (b *RecordBuilder) buildWith(builder *flatbuffers.Builder) {
	functionCall := builder.CreateByteVector(b.functionCall)
	logsegment.RecordStart(builder)
	logsegment.RecordAddTimestamp(builder, b.timestamp)
	logsegment.RecordAddFunctionId(builder, b.functionID)
	logsegment.RecordAddFunctionCall(builder, functionCall)
	logsegment.FinishSizePrefixedRecordBuffer(builder, logsegment.RecordEnd(builder))
}
