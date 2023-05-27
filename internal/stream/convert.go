package stream

func ConvertReader[To, From any](base Reader[From], conv func(From) (To, error)) Reader[To] {
	return &convertReader[To, From]{base: base, conv: conv}
}

type convertReader[To, From any] struct {
	base Reader[From]
	from []From
	conv func(From) (To, error)
}

func (r *convertReader[To, From]) Read(values []To) (n int, err error) {
	for n < len(values) {
		if i := len(values) - n; i <= cap(r.from) {
			r.from = r.from[:i]
		} else {
			r.from = make([]From, i)
		}

		rn, err := r.base.Read(r.from)

		for _, from := range r.from[:rn] {
			to, err := r.conv(from)
			if err != nil {
				return n, err
			}
			values[n] = to
			n++
		}

		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func ConvertWriter[To, From any](base Writer[To], conv func(From) (To, error)) Writer[From] {
	return &convertWriter[To, From]{base: base, conv: conv}
}

type convertWriter[To, From any] struct {
	base Writer[To]
	to   []To
	conv func(From) (To, error)
}

func (w *convertWriter[To, From]) Write(values []From) (n int, err error) {
	defer func() {
		w.to = w.to[:0]
	}()

	for _, from := range values {
		to, err := w.conv(from)
		if err != nil {
			return 0, err
		}
		w.to = append(w.to, to)
	}

	return w.base.Write(w.to)
}
