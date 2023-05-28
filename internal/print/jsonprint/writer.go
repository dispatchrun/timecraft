package jsonprint

import (
	"encoding/json"
	"io"

	"github.com/stealthrocket/timecraft/internal/stream"
)

func NewWriter[T any](w io.Writer) stream.WriteCloser[T] {
	e := json.NewEncoder(w)
	e.SetEscapeHTML(false)
	e.SetIndent("", "  ")
	return writer[T]{e}
}

type writer[T any] struct{ *json.Encoder }

func (w writer[T]) Write(values []T) (int, error) {
	for n := range values {
		if err := w.Encode(values[n]); err != nil {
			return n, err
		}
	}
	return len(values), nil
}

func (w writer[T]) Close() error {
	return nil
}
