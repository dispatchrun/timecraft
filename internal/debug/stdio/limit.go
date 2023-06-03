package stdio

import (
	"bytes"
	"io"
)

type Limit struct {
	R io.Reader
	N int
}

func (r *Limit) Read(b []byte) (int, error) {
	if r.N <= 0 {
		return 0, io.EOF
	}

	n, err := r.R.Read(b)

	nl := bytes.Count(b[:n], []byte{'\n'})
	if nl <= r.N {
		r.N -= nl
		if r.N == 0 {
			n = bytes.LastIndexByte(b[:n], '\n') + 1
		}
		return n, err
	}

	for i, c := range b[:n] {
		if c != '\n' {
			continue
		}
		if r.N--; r.N == 0 {
			return i + 1, io.EOF
		}
	}

	return n, io.EOF
}
