package ioperf

import (
	"io"
)

func NewDoubleBufferedReader(reader io.ReadSeeker) io.ReadSeekCloser {
	bufs := make(chan []byte)
	errs := make(chan error, 1)
	done := make(chan []byte, 2)
	seek := make(chan seek)

	const bufferSize = 32 * 1024
	const split = bufferSize / 2
	buf := make([]byte, bufferSize)

	done <- buf[:split:split]
	done <- buf[split:]
	go doubleBufferedRead(reader, bufs, errs, done, seek)

	return &doubleBufferedReader{
		reader: reader,
		bufs:   bufs,
		errs:   errs,
		done:   done,
		seek:   seek,
	}
}

type doubleBufferedReader struct {
	reader io.ReadSeeker
	buffer []byte
	offset int

	bufs <-chan []byte
	errs <-chan error
	done chan<- []byte
	seek chan<- seek
}

type seek struct {
	offset int64
	whence int
	result chan<- int64
}

func (r *doubleBufferedReader) Close() error {
	if r.done != nil {
		close(r.done)
		r.done = nil
	}
	if r.seek != nil {
		close(r.seek)
		r.seek = nil
	}
	for range r.bufs {
	}
	for range r.errs {
	}
	return nil
}

func (r *doubleBufferedReader) Read(b []byte) (int, error) {
	if r.offset == len(r.buffer) {
		if r.done == nil {
			return 0, io.EOF
		}

		if r.buffer != nil {
			r.done <- r.buffer
			r.buffer = nil
			r.offset = 0
		}

		select {
		case buf, ok := <-r.bufs:
			if !ok {
				return 0, io.EOF
			}
			r.buffer = buf
			r.offset = 0
		case err, ok := <-r.errs:
			return 0, makeError(err, ok)
		}
	}

	n := copy(b, r.buffer[r.offset:])
	r.offset += n
	return n, nil
}

func (r *doubleBufferedReader) Seek(offset int64, whence int) (int64, error) {
	if r.seek == nil {
		return 0, io.EOF
	}

	result := make(chan int64, 1)
	select {
	case r.seek <- seek{offset, whence, result}:
	case err, ok := <-r.errs:
		return -1, makeError(err, ok)
	}

	select {
	case off := <-result:
		return off, nil
	case err, ok := <-r.errs:
		return -1, makeError(err, ok)
	}
}

func doubleBufferedRead(r io.ReadSeeker, bufs chan<- []byte, errs chan<- error, done <-chan []byte, seek <-chan seek) {
	defer close(errs)
	defer close(bufs)

	for {
		select {
		case b, ok := <-done:
			if !ok {
				return
			}
			n, err := r.Read(b[:cap(b)])
			if n > 0 {
				bufs <- b[:n]
			}
			if err != nil {
				errs <- err
				return
			}

		case s, ok := <-seek:
			if !ok {
				return
			}
			res, err := r.Seek(s.offset, s.whence)
			if err != nil {
				errs <- err
				return
			}
			s.result <- res
		}
	}
}

func makeError(err error, ok bool) error {
	if !ok {
		return io.EOF
	}
	return err
}
