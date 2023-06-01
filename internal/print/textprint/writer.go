package textprint

import (
	"bufio"
	"fmt"
	"io"

	"github.com/stealthrocket/timecraft/internal/stream"
)

const (
	format    = "%v"
	separator = "--------------------------------------------------------------------------------\n"
)

type WriterOption[T any] func(*writer[T])

func Format[T any](s string) WriterOption[T] {
	return func(w *writer[T]) { w.format = s }
}

func Separator[T any](s string) WriterOption[T] {
	return func(w *writer[T]) { w.separator = s }
}

func NewWriter[T any](w io.Writer, opts ...WriterOption[T]) stream.WriteCloser[T] {
	nw := &writer[T]{
		output:    bufio.NewWriter(w),
		format:    format,
		separator: separator,
	}
	for _, opt := range opts {
		opt(nw)
	}
	return nw
}

type writer[T any] struct {
	output    *bufio.Writer
	count     int
	format    string
	separator string
}

func (w *writer[T]) Write(values []T) (int, error) {
	for n, v := range values {
		if w.count++; w.count > 1 {
			_, err := io.WriteString(w.output, w.separator)
			if err != nil {
				return n, err
			}
		}
		_, err := fmt.Fprintf(w.output, w.format, v)
		if err != nil {
			return n, err
		}
	}
	return len(values), nil
}

func (w *writer[T]) Close() error {
	return w.output.Flush()
}
