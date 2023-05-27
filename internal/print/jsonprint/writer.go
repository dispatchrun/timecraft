package jsonprint

import (
	"bytes"
	"encoding/json"
	"io"

	"github.com/stealthrocket/timecraft/internal/stream"
)

func NewWriter[T any](w io.Writer) stream.WriteCloser[T] {
	b := new(bytes.Buffer)
	e := json.NewEncoder(b)
	e.SetEscapeHTML(false)
	e.SetIndent("  ", "  ")
	return &writer[T]{
		output:  w,
		buffer:  b,
		encoder: e,
	}
}

type writer[T any] struct {
	output  io.Writer
	buffer  *bytes.Buffer
	encoder *json.Encoder
	count   int
}

func (w *writer[T]) Write(values []T) (int, error) {
	if w.buffer == nil {
		return 0, io.ErrClosedPipe
	}
	if w.buffer.Len() == 0 {
		w.buffer.WriteString("[\n  ")
	}
	for n := range values {
		if w.count != 0 {
			w.buffer.WriteString(",\n  ")
		}
		if err := w.encoder.Encode(values[n]); err != nil {
			return n, err
		}
		w.count++
		w.buffer.Truncate(w.buffer.Len() - 1)
	}
	return len(values), nil
}

func (w *writer[T]) Close() (err error) {
	if w.buffer != nil {
		defer func() { w.buffer = nil }()

		if w.buffer.Len() == 0 {
			w.buffer.WriteString("[]\n")
		} else {
			w.buffer.WriteString("\n]\n")
		}

		_, err = w.buffer.WriteTo(w.output)
	}
	return err
}
