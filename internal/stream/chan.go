package stream

import "io"

type Optional[T any] struct {
	val T
	err error
}

func Opt[T any](val T, err error) Optional[T] {
	return Optional[T]{val: val, err: err}
}

func (opt Optional[T]) Value() (T, error) {
	return opt.val, opt.err
}

func ChanReader[T any](ch <-chan Optional[T]) Reader[T] {
	return chanReader[T](ch)
}

type chanReader[T any] <-chan Optional[T]

func (r chanReader[T]) Read(values []T) (int, error) {
	if len(values) == 0 {
		return 0, nil
	}

	opt, ok := <-r
	if !ok {
		return 0, io.EOF
	}
	v, err := opt.Value()
	if err != nil {
		return 0, err
	}
	values[0] = v

	for n := 1; n < len(values); n++ {
		select {
		case opt, ok := <-r:
			if !ok {
				return n, io.EOF
			}
			v, err := opt.Value()
			if err != nil {
				return n, err
			}
			values[n] = v
		default:
			return n, nil
		}
	}

	return len(values), nil
}
