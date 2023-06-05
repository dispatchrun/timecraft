package textprint

import (
	"bytes"
	"io"
)

type lineprefixer struct {
	p []byte
	w io.Writer

	start bool
}

func Prefixlines(w io.Writer, prefix []byte) io.Writer {
	return &lineprefixer{
		p: prefix,
		w: w,

		start: true,
	}
}

func (l *lineprefixer) Write(b []byte) (int, error) {
	count := 0
	for len(b) > 0 {
		if l.start {
			l.start = false
			n, err := l.w.Write(l.p)
			count += n
			if err != nil {
				return count, err
			}
		}

		i := bytes.IndexByte(b, '\n')
		if i == -1 {
			i = len(b)
		} else {
			i++ // include \n
			l.start = true
		}
		n, err := l.w.Write(b[:i])
		count += n
		if err != nil {
			return count, err
		}
		b = b[i:]
	}
	return count, nil
}

func Nolastline(w io.Writer) io.Writer {
	return &nolastliner{w: w}
}

type nolastliner struct {
	w   io.Writer
	has bool
}

func (l *nolastliner) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	count := 0
	if l.has {
		l.has = false
		n, err := l.w.Write([]byte{'\n'})
		count += n
		if err != nil {
			return count, err
		}
	}
	i := bytes.LastIndexByte(b, '\n')
	if i == len(b)-1 {
		l.has = true
		b = b[:i]
	}
	n, err := l.w.Write(b)
	count += n
	return count, err
}
