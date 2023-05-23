package timemachine_test

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/functioncall"
)

func TestReadHeader(t *testing.T) {
	b := new(bytes.Buffer)
	w := timemachine.NewLogWriter(b)

	header := &timemachine.Header{
		Runtime: timemachine.Runtime{
			Runtime: "test",
			Version: "dev",
			Functions: []timemachine.Function{
				{Module: "env", Name: "f0", ParamCount: 1, ResultCount: 1},
				{Module: "env", Name: "f1", ParamCount: 2, ResultCount: 1},
				{Module: "env", Name: "f2", ParamCount: 3, ResultCount: 1},
				{Module: "env", Name: "f3", ParamCount: 0, ResultCount: 0},
				{Module: "env", Name: "f4", ParamCount: 1, ResultCount: 1},
			},
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

	for i := 0; i < 10; i++ {
		h, _, err := r1.ReadLogHeader()
		if err != nil {
			t.Fatal(err)
		}
		if diff := cmp.Diff(header, h); diff != "" {
			t.Fatal(diff)
		}
	}
}

func TestReadRecordBatch(t *testing.T) {
	header := &timemachine.Header{
		Runtime: timemachine.Runtime{
			Runtime: "test",
			Version: "dev",
			Functions: []timemachine.Function{
				{Module: "env", Name: "f0", ParamCount: 1, ResultCount: 1},
				{Module: "env", Name: "f1", ParamCount: 2, ResultCount: 1},
				{Module: "env", Name: "f2", ParamCount: 3, ResultCount: 1},
				{Module: "env", Name: "f3", ParamCount: 0, ResultCount: 0},
				{Module: "env", Name: "f4", ParamCount: 1, ResultCount: 1},
			},
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
				Timestamp:  header.Process.StartTime.Add(1 * time.Millisecond),
				FunctionID: 0,
				Params:     []uint64{1},
				Results:    []uint64{42},
				MemoryAccess: []functioncall.MemoryAccess{
					{Memory: []byte("hello world!"), Offset: 1234},
				},
			},
		},
		{
			{
				Timestamp:  header.Process.StartTime.Add(2 * time.Millisecond),
				FunctionID: 1,
				Params:     []uint64{1, 2},
				Results:    []uint64{42},
			},
			{
				Timestamp:  header.Process.StartTime.Add(3 * time.Millisecond),
				FunctionID: 2,
				Params:     []uint64{1, 2, 3},
				Results:    []uint64{42},
			},
		},
		{
			{
				Timestamp:  header.Process.StartTime.Add(4 * time.Millisecond),
				FunctionID: 3,
				MemoryAccess: []functioncall.MemoryAccess{
					{Memory: []byte("A"), Offset: 1},
					{Memory: []byte("B"), Offset: 2},
					{Memory: []byte("C"), Offset: 3},
					{Memory: []byte("D"), Offset: 4},
				},
			},
			{
				Timestamp:  header.Process.StartTime.Add(5 * time.Millisecond),
				FunctionID: 4,
				Params:     []uint64{1},
				Results:    []uint64{42},
				MemoryAccess: []functioncall.MemoryAccess{
					{Memory: []byte("hello world!"), Offset: 1234},
					{Memory: make([]byte, 10e3), Offset: 1234567},
				},
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
	var functionCallBuilder functioncall.FunctionCallBuilder
	var recordBuilder timemachine.RecordBuilder
	var recordBatchBuilder timemachine.RecordBatchBuilder
	var firstOffset int64
	for _, batch := range batches {
		recordBatchBuilder.Reset(header.Compression, firstOffset)
		for _, r := range batch {
			functionCallBuilder.Reset(&header.Runtime.Functions[r.FunctionID])
			functionCallBuilder.SetParams(r.Params)
			functionCallBuilder.SetResults(r.Results)
			for _, m := range r.MemoryAccess {
				functionCallBuilder.AddMemoryAccess(m)
			}
			recordBuilder.Reset(header.Process.StartTime)
			recordBuilder.SetTimestamp(r.Timestamp)
			recordBuilder.SetFunctionID(r.FunctionID)
			recordBuilder.SetFunctionCall(functionCallBuilder.Bytes())
			recordBatchBuilder.AddRecord(&recordBuilder)
		}
		if err := writer.WriteRecordBatch(&recordBatchBuilder); err != nil {
			t.Fatal(err)
		}
		firstOffset += int64(len(batch))
	}

	reader := timemachine.NewLogReader(bytes.NewReader(buffer.Bytes()))

	headerRead, offset, err := reader.ReadLogHeader()
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(header, headerRead); diff != "" {
		t.Fatal(diff)
	}

	batchesRead := make([][]record, 0, len(batches))
	for {
		batch, length, err := reader.ReadRecordBatch(headerRead, offset)
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		records := make([]record, batch.NumRecords())
		count := 0
		for batch.Next() {
			r, err := batch.Record()
			if err != nil {
				t.Fatal(err)
			}
			function, err := r.Function()
			if err != nil {
				t.Fatal(err)
			}
			f := functioncall.MakeFunctionCall(function, r.FunctionCall())
			var params []uint64
			if f.NumParams() > 0 {
				params = make([]uint64, f.NumParams())
				for i := range params {
					params[i] = f.Param(i)
				}
			}
			var results []uint64
			if f.NumResults() > 0 {
				results = make([]uint64, f.NumResults())
				for i := range results {
					results[i] = f.Result(i)
				}
			}
			var memoryAccess []functioncall.MemoryAccess
			if f.NumMemoryAccess() > 0 {
				memoryAccess = make([]functioncall.MemoryAccess, f.NumMemoryAccess())
				for i := range memoryAccess {
					memoryAccess[i] = f.MemoryAccess(i)
				}
			}
			if count >= len(records) {
				t.Fatal("too many records")
			}
			records[count] = record{
				Timestamp:    r.Timestamp(),
				FunctionID:   r.FunctionID(),
				Params:       params,
				Results:      results,
				MemoryAccess: memoryAccess,
			}
			count++
		}
		if count != len(records) {
			t.Fatal("not enough records")
		}
		batchesRead = append(batchesRead, records)
		offset += length
	}
	if diff := cmp.Diff(batches, batchesRead); diff != "" {
		t.Fatal(diff)
	}
}

type record struct {
	Timestamp    time.Time
	FunctionID   int
	Params       []uint64
	Results      []uint64
	MemoryAccess []functioncall.MemoryAccess
}

func BenchmarkLogReader(b *testing.B) {
	buffer := new(bytes.Buffer)
	writer := timemachine.NewLogWriter(buffer)

	header := &timemachine.Header{
		Runtime: timemachine.Runtime{
			Runtime: "test",
			Version: "dev",
			Functions: []timemachine.Function{
				{Module: "env", Name: "f0"},
				{Module: "env", Name: "f1"},
				{Module: "env", Name: "f2"},
				{Module: "env", Name: "f3"},
				{Module: "env", Name: "f4"},
			},
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

	if err := writer.WriteLogHeader(&headerBuilder); err != nil {
		b.Fatal(err)
	}

	b.Run("ReadLogHeader", func(b *testing.B) {
		r0 := bytes.NewReader(buffer.Bytes())
		r1 := timemachine.NewLogReader(r0)

		for i := 0; i < b.N; i++ {
			_, _, err := r1.ReadLogHeader()
			if err != nil {
				b.Fatal(i, err)
			}
		}
	})
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
						Functions: []timemachine.Function{
							{Module: "env", Name: "f0", ParamCount: 1, ResultCount: 1},
							{Module: "env", Name: "f1", ParamCount: 2, ResultCount: 1},
							{Module: "env", Name: "f2", ParamCount: 3, ResultCount: 1},
							{Module: "env", Name: "f3", ParamCount: 0, ResultCount: 0},
							{Module: "env", Name: "f4", ParamCount: 1, ResultCount: 1},
						},
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
				Functions: []timemachine.Function{
					{Module: "env", Name: "f0", ParamCount: 1, ResultCount: 1},
					{Module: "env", Name: "f1", ParamCount: 2, ResultCount: 1},
					{Module: "env", Name: "f2", ParamCount: 3, ResultCount: 1},
					{Module: "env", Name: "f3", ParamCount: 0, ResultCount: 0},
					{Module: "env", Name: "f4", ParamCount: 1, ResultCount: 1},
				},
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
						Timestamp:  header.Process.StartTime.Add(1 * time.Millisecond),
						FunctionID: 0,
						Params:     []uint64{1},
						Results:    []uint64{42},
						MemoryAccess: []functioncall.MemoryAccess{
							{Memory: []byte("hello world!"), Offset: 1234},
						},
					},
				},
			},

			{
				scenario: "five records",
				batch: []record{
					{
						Timestamp:  header.Process.StartTime.Add(1 * time.Millisecond),
						FunctionID: 0,
						Params:     []uint64{1},
						Results:    []uint64{42},
						MemoryAccess: []functioncall.MemoryAccess{
							{Memory: []byte("hello world!"), Offset: 1234},
						},
					},
					{
						Timestamp:  header.Process.StartTime.Add(2 * time.Millisecond),
						FunctionID: 1,
						Params:     []uint64{1, 2},
						Results:    []uint64{42},
					},
					{
						Timestamp:  header.Process.StartTime.Add(3 * time.Millisecond),
						FunctionID: 2,
						Params:     []uint64{1, 2, 3},
						Results:    []uint64{42},
					},
					{
						Timestamp:  header.Process.StartTime.Add(4 * time.Millisecond),
						FunctionID: 3,
						MemoryAccess: []functioncall.MemoryAccess{
							{Memory: []byte("A"), Offset: 1},
							{Memory: []byte("B"), Offset: 2},
							{Memory: []byte("C"), Offset: 3},
							{Memory: []byte("D"), Offset: 4},
						},
					},
					{
						Timestamp:  header.Process.StartTime.Add(5 * time.Millisecond),
						FunctionID: 4,
						Params:     []uint64{1},
						Results:    []uint64{42},
						MemoryAccess: []functioncall.MemoryAccess{
							{Memory: []byte("hello world!"), Offset: 1234},
							{Memory: make([]byte, 10e3), Offset: 1234567},
						},
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

	var functionCallBuilder functioncall.FunctionCallBuilder
	var recordBuilder timemachine.RecordBuilder
	var recordBatchBuilder timemachine.RecordBatchBuilder

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		recordBatchBuilder.Reset(header.Compression, 0)
		for _, r := range batch {
			functionCallBuilder.Reset(&header.Runtime.Functions[r.FunctionID])
			functionCallBuilder.SetParams(r.Params)
			functionCallBuilder.SetResults(r.Results)
			for _, m := range r.MemoryAccess {
				functionCallBuilder.AddMemoryAccess(m)
			}
			recordBuilder.Reset(header.Process.StartTime)
			recordBuilder.SetTimestamp(r.Timestamp)
			recordBuilder.SetFunctionID(r.FunctionID)
			recordBuilder.SetFunctionCall(functionCallBuilder.Bytes())
			recordBatchBuilder.AddRecord(&recordBuilder)
		}
		if err := w.WriteRecordBatch(&recordBatchBuilder); err != nil {
			b.Fatal(err)
		}
	}
}
