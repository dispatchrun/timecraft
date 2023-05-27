// Package stream is a library of generic types designed to work on streams of
// values.
package stream

import "io"

// Reader is an interface implemented by types that produce a stream of values
// of type T.
type Reader[T any] interface {
	// Reads values from the stream, returning the number of values read and any
	// error that occurred.
	//
	// The error is io.EOF when the end of the stream has been reached.
	Read(values []T) (int, error)
}

// NewReader constructs a Reader from a sequence of values.
func NewReader[T any](values ...T) Reader[T] {
	return &reader[T]{values: append([]T{}, values...)}
}

type reader[T any] struct{ values []T }

func (r *reader[T]) Read(values []T) (n int, err error) {
	n = copy(values, r.values)
	r.values = r.values[n:]
	if len(r.values) == 0 {
		err = io.EOF
	}
	return n, err
}

// ReadCloser represents a closable stream of values of T.
//
// ReadClosers is like io.ReadCloser for values of any type.
type ReadCloser[T any] interface {
	Reader[T]
	io.Closer
}

// NewReadCloser constructs a ReadCloser from the pair of r and c.
func NewReadCloser[T any](r Reader[T], c io.Closer) ReadCloser[T] {
	return &readCloser[T]{reader: r, closer: c}
}

type readCloser[T any] struct {
	reader Reader[T]
	closer io.Closer
}

func (r *readCloser[T]) Close() error                 { return r.closer.Close() }
func (r *readCloser[T]) Read(values []T) (int, error) { return r.reader.Read(values) }

// NopCloser constructs a ReadCloser from a Reader.
func NopCloser[T any](r Reader[T]) ReadCloser[T] {
	return &nopCloser[T]{reader: r}
}

type nopCloser[T any] struct{ reader Reader[T] }

func (r *nopCloser[T]) Close() error                 { return nil }
func (r *nopCloser[T]) Read(values []T) (int, error) { return r.reader.Read(values) }

// ReadAll reads all values from r and returns them as a slice, along with any
// error that occurred (other than io.EOF).
func ReadAll[T any](r Reader[T]) ([]T, error) {
	values := make([]T, 0, 1)
	for {
		if len(values) == cap(values) {
			values = append(values, make([]T, 2*len(values))...)[:len(values)]
		}
		n, err := r.Read(values[len(values):cap(values)])
		values = values[:len(values)+n]
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return values, err
		}
	}
}
