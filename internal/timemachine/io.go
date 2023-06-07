package timemachine

import (
	"bufio"
	"io"
)

type bufferedReadSeeker struct {
	buffer *bufio.Reader
	reader io.ReadSeeker
	offset int64
}

func newBufferedReadSeeker(input io.ReadSeeker, bufferSize int) *bufferedReadSeeker {
	return &bufferedReadSeeker{
		buffer: bufio.NewReaderSize(input, bufferSize),
		reader: input,
	}
}

func (r *bufferedReadSeeker) Read(b []byte) (int, error) {
	n, err := r.buffer.Read(b)
	r.offset += int64(n)
	return n, err
}

func (r *bufferedReadSeeker) Seek(offset int64, whence int) (int64, error) {
	offset, err := r.reader.Seek(offset, whence)
	if err != nil {
		return -1, err
	}
	endBufferedOffset := r.offset + int64(r.buffer.Buffered())
	if offset >= r.offset && offset < endBufferedOffset {
		_, _ = r.buffer.Discard(int(offset - r.offset))
	} else {
		r.buffer.Reset(r.reader)
	}
	r.offset = offset
	return offset, nil
}

var (
	_ io.ReadSeeker = (*bufferedReadSeeker)(nil)
)
