package timemachine_test

import (
	"bytes"
	"io"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

func TestReadRecordBatch(t *testing.T) {
	startTime := time.Now()

	batches := [][]record{
		{
			{
				Timestamp:    startTime.Add(1 * time.Millisecond),
				FunctionID:   0,
				FunctionCall: []byte("function call 0"),
			},
		},
		{
			{
				Timestamp:    startTime.Add(2 * time.Millisecond),
				FunctionID:   1,
				FunctionCall: []byte("function call 1"),
			},
			{
				Timestamp:    startTime.Add(3 * time.Millisecond),
				FunctionID:   2,
				FunctionCall: []byte("function call 2"),
			},
		},
		{
			{
				Timestamp:    startTime.Add(4 * time.Millisecond),
				FunctionID:   3,
				FunctionCall: []byte("function call: A, B, C, D"),
			},
			{
				Timestamp:    startTime.Add(5 * time.Millisecond),
				FunctionID:   4,
				FunctionCall: []byte("hello world!"),
			},
		},
	}

	buffer := new(bytes.Buffer)
	writer := timemachine.NewLogWriter(buffer)

	var recordBuilder timemachine.RecordBuilder
	var recordBatchBuilder timemachine.RecordBatchBuilder
	var firstOffset int64
	for _, batch := range batches {
		recordBatchBuilder.Reset(timemachine.Zstd, firstOffset)
		for _, r := range batch {
			recordBuilder.Reset(startTime)
			recordBuilder.SetTimestamp(r.Timestamp)
			recordBuilder.SetFunctionID(r.FunctionID)
			recordBuilder.SetFunctionCall(r.FunctionCall)
			recordBatchBuilder.AddRecord(&recordBuilder)
		}
		if err := writer.WriteRecordBatch(&recordBatchBuilder); err != nil {
			t.Fatal(err)
		}
		firstOffset += int64(len(batch))
	}

	reader := timemachine.NewLogReader(bytes.NewReader(buffer.Bytes()), startTime)
	batchesRead := make([][]record, 0, len(batches))
	for {
		batch, err := reader.ReadRecordBatch()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		records := make([]record, batch.NumRecords())
		count := 0
		iter := stream.Iter[timemachine.Record](batch)

		for iter.Next() {
			r := iter.Value()
			assert.Less(t, count, len(records))
			records[count] = record{
				Timestamp:    r.Timestamp(),
				FunctionID:   r.FunctionID(),
				FunctionCall: r.FunctionCall(),
			}
			count++
		}

		assert.OK(t, iter.Err())
		assert.Equal(t, count, len(records))
		batchesRead = append(batchesRead, records)
	}

	if diff := cmp.Diff(batches, batchesRead); diff != "" {
		t.Fatal(diff)
	}
}

type record struct {
	Timestamp    time.Time
	FunctionID   int
	FunctionCall []byte
}

func BenchmarkLogWriter(b *testing.B) {
	startTime := time.Now()

	tests := []struct {
		scenario string
		batch    []record
	}{
		{
			scenario: "zero records",
		},

		{
			scenario: "one record",
			batch: []record{
				{
					Timestamp:    startTime.Add(1 * time.Millisecond),
					FunctionID:   0,
					FunctionCall: []byte("function call 0"),
				},
			},
		},

		{
			scenario: "five records",
			batch: []record{
				{
					Timestamp:    startTime.Add(1 * time.Millisecond),
					FunctionID:   0,
					FunctionCall: []byte("1"),
				},
				{
					Timestamp:    startTime.Add(2 * time.Millisecond),
					FunctionID:   1,
					FunctionCall: []byte("1,2"),
				},
				{
					Timestamp:    startTime.Add(3 * time.Millisecond),
					FunctionID:   2,
					FunctionCall: []byte("1,2,3"),
				},
				{
					Timestamp:    startTime.Add(4 * time.Millisecond),
					FunctionID:   3,
					FunctionCall: []byte("A,B,C,D"),
				},
				{
					Timestamp:    startTime.Add(5 * time.Millisecond),
					FunctionID:   4,
					FunctionCall: []byte("hello world!"),
				},
			},
		},
	}

	for _, test := range tests {
		b.Run(test.scenario, func(b *testing.B) {
			benchmarkLogWriterWriteRecordBatch(b, startTime, timemachine.Zstd, test.batch)
		})
	}
}

func benchmarkLogWriterWriteRecordBatch(b *testing.B, startTime time.Time, compression timemachine.Compression, batch []record) {
	w := timemachine.NewLogWriter(io.Discard)

	var recordBuilder timemachine.RecordBuilder
	var recordBatchBuilder timemachine.RecordBatchBuilder

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		recordBatchBuilder.Reset(compression, 0)
		for _, r := range batch {
			recordBuilder.Reset(startTime)
			recordBuilder.SetTimestamp(r.Timestamp)
			recordBuilder.SetFunctionID(r.FunctionID)
			recordBuilder.SetFunctionCall(r.FunctionCall)
			recordBatchBuilder.AddRecord(&recordBuilder)
		}
		if err := w.WriteRecordBatch(&recordBatchBuilder); err != nil {
			b.Fatal(err)
		}
	}
}
