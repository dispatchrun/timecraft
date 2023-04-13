package timelog_test

import (
	"io"
	"os"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/pkg/timelog"
)

func BenchmarkLogWriter(b *testing.B) {
	b.Run("WriteLogHeader", func(b *testing.B) {
		tests := []struct {
			scenario string
			header   *timelog.LogHeader
		}{
			{
				scenario: "common log header",
				header: &timelog.LogHeader{
					Runtime: timelog.Runtime{
						Runtime: "test",
						Version: "dev",
						Functions: []timelog.Function{
							{Module: "env", Name: "f0"},
							{Module: "env", Name: "f1"},
							{Module: "env", Name: "f2"},
							{Module: "env", Name: "f3"},
							{Module: "env", Name: "f4"},
						},
					},
					Process: timelog.Process{
						ID:        timelog.Hash{"sha", "f572d396fae9206628714fb2ce00f72e94f2258f"},
						Image:     timelog.Hash{"sha", "28935580a9bbb8cc7bcdea62e7dfdcf7e0f31f87"},
						StartTime: time.Now(),
						Args:      os.Args,
						Environ:   os.Environ(),
					},
					Segment:     42,
					Compression: timelog.Zstd,
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
		header := &timelog.LogHeader{
			Runtime: timelog.Runtime{
				Runtime: "test",
				Version: "dev",
				Functions: []timelog.Function{
					{Module: "env", Name: "f0"},
					{Module: "env", Name: "f1"},
					{Module: "env", Name: "f2"},
					{Module: "env", Name: "f3"},
					{Module: "env", Name: "f4"},
				},
			},
			Process: timelog.Process{
				ID:        timelog.Hash{"sha", "f572d396fae9206628714fb2ce00f72e94f2258f"},
				Image:     timelog.Hash{"sha", "28935580a9bbb8cc7bcdea62e7dfdcf7e0f31f87"},
				StartTime: time.Now(),
				Args:      os.Args,
				Environ:   os.Environ(),
			},
			Segment:     42,
			Compression: timelog.Zstd,
		}

		tests := []struct {
			scenario string
			batch    []timelog.Record
		}{
			{
				scenario: "zero records",
			},

			{
				scenario: "one record",
				batch: []timelog.Record{
					{
						Timestamp: header.Process.StartTime.Add(1 * time.Millisecond),
						Function:  timelog.Function{Module: "env", Name: "f0"},
						Params:    []uint64{1},
						Results:   []uint64{42},
						MemoryAccess: []timelog.MemoryAccess{
							{Memory: []byte("hello world!"), Offset: 1234, Access: timelog.MemoryRead},
						},
					},
				},
			},

			{
				scenario: "five records",
				batch: []timelog.Record{
					{
						Timestamp: header.Process.StartTime.Add(1 * time.Millisecond),
						Function:  timelog.Function{Module: "env", Name: "f0"},
						Params:    []uint64{1},
						Results:   []uint64{42},
						MemoryAccess: []timelog.MemoryAccess{
							{Memory: []byte("hello world!"), Offset: 1234, Access: timelog.MemoryRead},
						},
					},
					{
						Timestamp: header.Process.StartTime.Add(2 * time.Millisecond),
						Function:  timelog.Function{Module: "env", Name: "f1"},
						Params:    []uint64{1, 2},
						Results:   []uint64{42},
					},
					{
						Timestamp: header.Process.StartTime.Add(3 * time.Millisecond),
						Function:  timelog.Function{Module: "env", Name: "f2"},
						Params:    []uint64{1, 2, 3},
						Results:   []uint64{42},
					},
					{
						Timestamp: header.Process.StartTime.Add(4 * time.Millisecond),
						Function:  timelog.Function{Module: "env", Name: "f3"},
						MemoryAccess: []timelog.MemoryAccess{
							{Memory: []byte("A"), Offset: 1, Access: timelog.MemoryRead},
							{Memory: []byte("B"), Offset: 2, Access: timelog.MemoryRead},
							{Memory: []byte("C"), Offset: 3, Access: timelog.MemoryRead},
							{Memory: []byte("D"), Offset: 4, Access: timelog.MemoryRead},
						},
					},
					{
						Timestamp: header.Process.StartTime.Add(5 * time.Millisecond),
						Function:  timelog.Function{Module: "env", Name: "f4"},
						Params:    []uint64{1},
						Results:   []uint64{42},
						MemoryAccess: []timelog.MemoryAccess{
							{Memory: []byte("hello world!"), Offset: 1234, Access: timelog.MemoryRead},
							{Memory: make([]byte, 10e3), Offset: 1234567, Access: timelog.MemoryWrite},
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

func benchmarkLogWriterWriteLogHeader(b *testing.B, header *timelog.LogHeader) {
	w := timelog.NewLogWriter(io.Discard)

	for i := 0; i < b.N; i++ {
		if err := w.WriteLogHeader(header); err != nil {
			b.Fatal(err)
		}
		w.Reset(io.Discard)
	}
}

func benchmarkLogWriterWriteRecordBatch(b *testing.B, header *timelog.LogHeader, batch []timelog.Record) {
	w := timelog.NewLogWriter(io.Discard)
	w.WriteLogHeader(header)

	for i := 0; i < b.N; i++ {
		if err := w.WriteRecordBatch(batch); err != nil {
			b.Fatal(err)
		}
	}
}
