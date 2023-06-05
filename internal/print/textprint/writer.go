package textprint

import (
	"bufio"
	"fmt"
	"io"

	"github.com/stealthrocket/timecraft/internal/stream"
)

const (
	separator = "--------------------------------------------------------------------------------\n"
)

func NewWriter[T any](w io.Writer, format string) stream.WriteCloser[T] {
	return &writer[T]{
		output: bufio.NewWriter(w),
		format: format,
	}
}

type writer[T any] struct {
	output *bufio.Writer
	format string
	count  int
}

func (w *writer[T]) Write(values []T) (int, error) {
	for n, v := range values {
		if w.count++; w.count > 1 {
			if _, err := io.WriteString(w.output, separator); err != nil {
				return n, err
			}
		}
		if _, err := fmt.Fprintf(w.output, w.format, v); err != nil {
			return n, err
		}
	}
	return len(values), nil
}

func (w *writer[T]) Close() error {
	return w.output.Flush()
}
