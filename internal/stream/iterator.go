package stream

import (
	"io"
)

type Iterator[T any] struct {
	base ReadCloser[T]
	err  error
	off  int
	len  int
	buf  [20]T
}

func Items[T any](it *Iterator[T]) ([]T, error) {
	var items []T
	for it.Next() {
		items = append(items, it.Item())
	}
	return items, it.Close()
}

func Iter[T any](r ReadCloser[T]) *Iterator[T] {
	return &Iterator[T]{base: r}
}

func (it *Iterator[T]) Reset(r ReadCloser[T]) {
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

func (it *Iterator[T]) Close() error {
	err := it.base.Close()
	if err == nil && it.err != io.EOF {
		err = it.err
	}
	return err
}

func (it *Iterator[T]) Next() bool {
	if it.off++; it.off < it.len {
		return true
	}
	if it.base == nil {
		return false
	}
	n, err := it.base.Read(it.buf[:])
	it.err = err
	it.off = 0
	it.len = n
	return n > 0
}

func (it *Iterator[T]) Item() T {
	return it.buf[it.off]
}
