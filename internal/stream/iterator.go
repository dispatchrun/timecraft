package stream

import "io"

type Iterator[T any] struct {
	base Reader[T]
	err  error
	off  int
	buf  []T
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
	it.off = 0
	it.buf = it.buf[:0]
}

func (it *Iterator[T]) Next() bool {
	if it.off++; it.off < len(it.buf) {
		return true
	}
	return it.next()
}

// This is split out of Next so the hot code path incrementing the iterator
// offset and checking if we exhausted all the buffered values can be inlined.
func (it *Iterator[T]) next() bool {
	if it.base == nil {
		return false
	}
	if it.err != nil {
		return false
	}
	if cap(it.buf) == 0 {
		it.buf = make([]T, 256)
	}
	for {
		n, err := it.base.Read(it.buf[:cap(it.buf)])
		it.err = err
		it.off = 0
		it.buf = it.buf[:n]
		if n > 0 {
			return true
		}
		if err != nil {
			return false
		}
	}
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
