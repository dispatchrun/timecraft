package funccall

import (
	"encoding/binary"
	"errors"
	"io"
	"slices"
	"time"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

type Trace struct {
	Start    time.Time
	Duration time.Duration
	Depth    int
	Name     string
}

type TraceReader struct {
	Records stream.Reader[timemachine.Record]
	records []timemachine.Record
}

func (r *TraceReader) Read(traces []Trace) (n int, err error) {
	r.records = slices.Grow(r.records, len(traces))[:len(traces)]
	for {
		rn, err := stream.ReadFull(r.Records, r.records)
		for i := 0; i < rn; i++ {
			record := &r.records[i]
			if record.FunctionID == ID {
				// read depth
				data := record.FunctionCall
				depth := binary.BigEndian.Uint32(data)
				data = data[4:]
				//read duration
				duration := binary.BigEndian.Uint64(data)
				name := data[8:]
				traces[n] = Trace{
					Start:    record.Time,
					Duration: time.Duration(duration),
					Depth:    int(depth),
					Name:     string(name),
				}
				n++
			}
		}
		if n > 0 || err != nil {
			if errors.Is(err, io.ErrUnexpectedEOF) {
				err = io.EOF
			}
			return n, err
		}
	}
}
