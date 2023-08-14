package stream

import (
	"io"
	"slices"
)

func MultiReader[T any](readers ...Reader[T]) Reader[T] {
	return &multiReader[T]{readers: slices.Clone(readers)}
}

type multiReader[T any] struct {
	readers []Reader[T]
}

func (m *multiReader[T]) Read(values []T) (n int, err error) {
	for len(m.readers) > 0 && len(values) > 0 {
		rn, err := m.readers[0].Read(values)
		values = values[rn:]
		n += rn
		if err != nil {
			if err != io.EOF {
				return n, err
			}
			m.readers = m.readers[1:]
		}
		if rn == 0 {
			return n, io.ErrNoProgress
		}
	}
	if len(m.readers) == 0 {
		return n, io.EOF
	}
	return n, nil
}
