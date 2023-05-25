package timemachine_test

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

func TestReadHeader(t *testing.T) {
	b := new(bytes.Buffer)
	w := timemachine.NewLogWriter(b)

	header := &timemachine.Header{
		Runtime: timemachine.Runtime{
			Runtime: "test",
			Version: "dev",
		},
		Process: timemachine.Process{
			ID:        timemachine.Hash{"sha", "f572d396fae9206628714fb2ce00f72e94f2258f"},
			Image:     timemachine.Hash{"sha", "28935580a9bbb8cc7bcdea62e7dfdcf7e0f31f87"},
			StartTime: time.Now(),
			Args:      os.Args,
			Environ:   os.Environ(),
		},
		Segment:     42,
		Compression: timemachine.Zstd,
	}

	var headerBuilder timemachine.HeaderBuilder
	headerBuilder.SetProcess(header.Process)
	headerBuilder.SetRuntime(header.Runtime)
	headerBuilder.SetCompression(header.Compression)
	headerBuilder.SetSegment(header.Segment)

	if err := w.WriteLogHeader(&headerBuilder); err != nil {
		t.Fatal(err)
	}

	r0 := bytes.NewReader(b.Bytes())
	r1 := timemachine.NewLogReader(r0)

	h, err := r1.ReadLogHeader()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(header, h); diff != "" {
		t.Fatal(diff)
	}
}

func TestReadRecordBatch(t *testing.T) {
	header := &timemachine.Header{
		Runtime: timemachine.Runtime{
			Runtime: "test",
			Version: "dev",
		},
		Process: timemachine.Process{
			ID:        timemachine.Hash{"sha", "f572d396fae9206628714fb2ce00f72e94f2258f"},
			Image:     timemachine.Hash{"sha", "28935580a9bbb8cc7bcdea62e7dfdcf7e0f31f87"},
			StartTime: time.Now(),
			Args:      os.Args,
			Environ:   os.Environ(),
		},
		Segment:     42,
		Compression: timemachine.Zstd,
	}

	batches := [][]record{
		{
			{
				Timestamp:    header.Process.StartTime.Add(1 * time.Millisecond),
				FunctionID:   0,
				FunctionCall: []byte("function call 0"),
			},
		},
		{
			{
				Timestamp:    header.Process.StartTime.Add(2 * time.Millisecond),
				FunctionID:   1,
				FunctionCall: []byte("function call 1"),
			},
			{
				Timestamp:    header.Process.StartTime.Add(3 * time.Millisecond),
				FunctionID:   2,
				FunctionCall: []byte("function call 2"),
			},
		},
		{
			{
				Timestamp:    header.Process.StartTime.Add(4 * time.Millisecond),
				FunctionID:   3,
				FunctionCall: []byte("function call: A, B, C, D"),
			},
			{
				Timestamp:    header.Process.StartTime.Add(5 * time.Millisecond),
				FunctionID:   4,
				FunctionCall: []byte("hello world!"),
			},
		},
	}

	buffer := new(bytes.Buffer)
	writer := timemachine.NewLogWriter(buffer)

	var headerBuilder timemachine.HeaderBuilder
	headerBuilder.SetProcess(header.Process)
	headerBuilder.SetRuntime(header.Runtime)
	headerBuilder.SetCompression(header.Compression)
	headerBuilder.SetSegment(header.Segment)

	if err := writer.WriteLogHeader(&headerBuilder); err != nil {
		t.Fatal(err)
	}
	var recordBuilder timemachine.RecordBuilder
	var recordBatchBuilder timemachine.RecordBatchBuilder
	var firstOffset int64
	for _, batch := range batches {
		recordBatchBuilder.Reset(header.Compression, firstOffset)
		for _, r := range batch {
			recordBuilder.Reset(header.Process.StartTime)
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

	reader := timemachine.NewLogReader(bytes.NewReader(buffer.Bytes()))

	headerRead, err := reader.ReadLogHeader()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(header, headerRead); diff != "" {
		t.Fatal(diff)
	}

	batchesRead := make([][]record, 0, len(batches))
	for {
		batch, err := reader.ReadRecordBatch(headerRead)
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
	b.Run("WriteLogHeader", func(b *testing.B) {
		tests := []struct {
			scenario string
			header   *timemachine.Header
		}{
			{
				scenario: "common log header",
				header: &timemachine.Header{
					Runtime: timemachine.Runtime{
						Runtime: "test",
						Version: "dev",
					},
					Process: timemachine.Process{
						ID:        timemachine.Hash{"sha", "f572d396fae9206628714fb2ce00f72e94f2258f"},
						Image:     timemachine.Hash{"sha", "28935580a9bbb8cc7bcdea62e7dfdcf7e0f31f87"},
						StartTime: time.Now(),
						Args:      os.Args,
						Environ:   os.Environ(),
					},
					Segment:     42,
					Compression: timemachine.Zstd,
				},
			},
		}

		for _, test := range tests {
			b.Run(test.scenario, func(b *testing.B) {
				benchmarkLogWriterWriteLogHeader(b, test.header)
			})
		}
	})

	b.Run("WriteRecordBatch", func(b *testing.B) {
		header := &timemachine.Header{
			Runtime: timemachine.Runtime{
				Runtime: "test",
				Version: "dev",
			},
			Process: timemachine.Process{
				ID:        timemachine.Hash{"sha", "f572d396fae9206628714fb2ce00f72e94f2258f"},
				Image:     timemachine.Hash{"sha", "28935580a9bbb8cc7bcdea62e7dfdcf7e0f31f87"},
				StartTime: time.Now(),
				Args:      os.Args,
				Environ:   os.Environ(),
			},
			Segment:     42,
			Compression: timemachine.Zstd,
		}

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
						Timestamp:    header.Process.StartTime.Add(1 * time.Millisecond),
						FunctionID:   0,
						FunctionCall: []byte("function call 0"),
					},
				},
			},

			{
				scenario: "five records",
				batch: []record{
					{
						Timestamp:    header.Process.StartTime.Add(1 * time.Millisecond),
						FunctionID:   0,
						FunctionCall: []byte("1"),
					},
					{
						Timestamp:    header.Process.StartTime.Add(2 * time.Millisecond),
						FunctionID:   1,
						FunctionCall: []byte("1,2"),
					},
					{
						Timestamp:    header.Process.StartTime.Add(3 * time.Millisecond),
						FunctionID:   2,
						FunctionCall: []byte("1,2,3"),
					},
					{
						Timestamp:    header.Process.StartTime.Add(4 * time.Millisecond),
						FunctionID:   3,
						FunctionCall: []byte("A,B,C,D"),
					},
					{
						Timestamp:    header.Process.StartTime.Add(5 * time.Millisecond),
						FunctionID:   4,
						FunctionCall: []byte("hello world!"),
					},
				},
			},
		}

		for _, test := range tests {
			b.Run(test.scenario, func(b *testing.B) {
				benchmarkLogWriterWriteRecordBatch(b, header, test.batch)
			})
		}
	})
}

func benchmarkLogWriterWriteLogHeader(b *testing.B, header *timemachine.Header) {
	w := timemachine.NewLogWriter(io.Discard)

	var headerBuilder timemachine.HeaderBuilder
	headerBuilder.SetProcess(header.Process)
	headerBuilder.SetRuntime(header.Runtime)
	headerBuilder.SetCompression(header.Compression)
	headerBuilder.SetSegment(header.Segment)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		headerBuilder.Reset()
		headerBuilder.SetProcess(header.Process)
		headerBuilder.SetRuntime(header.Runtime)
		headerBuilder.SetCompression(header.Compression)
		headerBuilder.SetSegment(header.Segment)
		if err := w.WriteLogHeader(&headerBuilder); err != nil {
			b.Fatal(err)
		}
		w.Reset(io.Discard)
	}
}

func benchmarkLogWriterWriteRecordBatch(b *testing.B, header *timemachine.Header, batch []record) {
	w := timemachine.NewLogWriter(io.Discard)
	var headerBuilder timemachine.HeaderBuilder
	headerBuilder.SetProcess(header.Process)
	headerBuilder.SetRuntime(header.Runtime)
	headerBuilder.SetCompression(header.Compression)
	headerBuilder.SetSegment(header.Segment)
	if err := w.WriteLogHeader(&headerBuilder); err != nil {
		b.Fatal(err)
	}

	var recordBuilder timemachine.RecordBuilder
	var recordBatchBuilder timemachine.RecordBatchBuilder

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		recordBatchBuilder.Reset(header.Compression, 0)
		for _, r := range batch {
			recordBuilder.Reset(header.Process.StartTime)
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
