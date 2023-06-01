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

	batches := [][]timemachine.Record{
		{
			{
				Time:         startTime.Add(1 * time.Millisecond),
				FunctionID:   0,
				FunctionCall: []byte("function call 0"),
			},
		},
		{
			{
				Time:         startTime.Add(2 * time.Millisecond),
				FunctionID:   1,
				FunctionCall: []byte("function call 1"),
			},
			{
				Time:         startTime.Add(3 * time.Millisecond),
				FunctionID:   2,
				FunctionCall: []byte("function call 2"),
			},
		},
		{
			{
				Time:         startTime.Add(4 * time.Millisecond),
				FunctionID:   3,
				FunctionCall: []byte("function call: A, B, C, D"),
			},
			{
				Time:         startTime.Add(5 * time.Millisecond),
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
			recordBuilder.SetTimestamp(r.Time)
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
	batchesRead := make([][]timemachine.Record, 0, len(batches))
	for {
		batch, err := reader.ReadRecordBatch()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		records := make([]timemachine.Record, batch.NumRecords())
		count := 0
		iter := stream.Iter[timemachine.Record](batch)

		for iter.Next() {
			r := iter.Value()
			assert.Less(t, count, len(records))
			records[count] = r
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

func BenchmarkLogWriter(b *testing.B) {
	startTime := time.Now()

	tests := []struct {
		scenario string
		batch    []timemachine.Record
	}{
		{
			scenario: "zero records",
		},

		{
			scenario: "one record",
			batch: []timemachine.Record{
				{
					Time:         startTime.Add(1 * time.Millisecond),
					FunctionID:   0,
					FunctionCall: []byte("function call 0"),
				},
			},
		},

		{
			scenario: "five records",
			batch: []timemachine.Record{
				{
					Time:         startTime.Add(1 * time.Millisecond),
					FunctionID:   0,
					FunctionCall: []byte("1"),
				},
				{
					Time:         startTime.Add(2 * time.Millisecond),
					FunctionID:   1,
					FunctionCall: []byte("1,2"),
				},
				{
					Time:         startTime.Add(3 * time.Millisecond),
					FunctionID:   2,
					FunctionCall: []byte("1,2,3"),
				},
				{
					Time:         startTime.Add(4 * time.Millisecond),
					FunctionID:   3,
					FunctionCall: []byte("A,B,C,D"),
				},
				{
					Time:         startTime.Add(5 * time.Millisecond),
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

func benchmarkLogWriterWriteRecordBatch(b *testing.B, startTime time.Time, compression timemachine.Compression, batch []timemachine.Record) {
	w := timemachine.NewLogWriter(io.Discard)

	var recordBuilder timemachine.RecordBuilder
	var recordBatchBuilder timemachine.RecordBatchBuilder

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		recordBatchBuilder.Reset(compression, 0)
		for _, r := range batch {
			recordBuilder.Reset(startTime)
			recordBuilder.SetTimestamp(r.Time)
			recordBuilder.SetFunctionID(r.FunctionID)
			recordBuilder.SetFunctionCall(r.FunctionCall)
			recordBatchBuilder.AddRecord(&recordBuilder)
		}
		if err := w.WriteRecordBatch(&recordBatchBuilder); err != nil {
			b.Fatal(err)
		}
	}
}
