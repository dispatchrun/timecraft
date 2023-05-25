package stream

import "io"

type Iterator[T any] struct {
	base Reader[T]
	err  error
	off  int
	len  int
	buf  [20]T
}

func Values[T any](it *Iterator[T]) ([]T, error) {
	var values []T
	for it.Next() {
		values = append(values, it.Value())
	}
	return values, it.Err()
}

func Iter[T any](r Reader[T]) *Iterator[T] {
	return &Iterator[T]{base: r}
}

func (it *Iterator[T]) Reset(r Reader[T]) {
	it.base = r
	it.err = nil
	it.len = 0
	it.off = 0

	var zero T
	clear := it.buf[:]
	for i := range clear {
		clear[i] = zero
	}
}

func (it *Iterator[T]) Next() bool {
	if it.off++; it.off < it.len {
		return true
	}
	if it.base == nil {
		return false
	}
	if it.err != nil {
		return false
	}
	n, err := it.base.Read(it.buf[:])
	it.err = err
	it.off = 0
	it.len = n
	return n > 0
}

func (it *Iterator[T]) Value() T {
	return it.buf[it.off]
}

func (it *Iterator[T]) Err() error {
	err := it.err
	if err == io.EOF {
		err = nil
	}
	return err
}
