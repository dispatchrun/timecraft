package stdio

import (
	"bytes"
	"io"
	"strings"
	"unicode/utf8"
)

type Buffer struct {
	output io.Writer
	buffer []byte
}

func NewBuffer(output io.Writer) *Buffer {
	return &Buffer{output: output}
}

func (buf *Buffer) WriteByte(b byte) error {
	buf.buffer = append(buf.buffer, b)
	if b == '\n' {
		return buf.Flush()
	}
	return nil
}

func (buf *Buffer) WriteRune(r rune) (n int, err error) {
	baseLength := len(buf.buffer)
	buf.buffer = utf8.AppendRune(buf.buffer, r)
	n = len(buf.buffer) - baseLength
	if r == '\n' {
		err = buf.Flush()
	}
	return n, err
}

func (buf *Buffer) Write(b []byte) (n int, err error) {
	buf.buffer = append(buf.buffer, b...)
	if bytes.IndexByte(b, '\n') >= 0 {
		err = buf.flushLines()
	}
	return len(b), err
}

func (buf *Buffer) WriteString(s string) (n int, err error) {
	buf.buffer = append(buf.buffer, s...)
	if strings.IndexByte(s, '\n') >= 0 {
		err = buf.flushLines()
	}
	return len(s), err
}

func (buf *Buffer) Flush() (err error) {
	if len(buf.buffer) > 0 {
		err = buf.flush(len(buf.buffer))
	}
	return err
}

func (buf *Buffer) flush(limit int) error {
	n, err := buf.output.Write(buf.buffer[:limit])
	buf.buffer = buf.buffer[:copy(buf.buffer, buf.buffer[n:])]
	return err
}

func (buf *Buffer) flushLines() error {
	return buf.flush(bytes.LastIndexByte(buf.buffer, '\n') + 1)
}

var (
	_ io.ByteWriter   = (*Buffer)(nil)
	_ io.StringWriter = (*Buffer)(nil)
	_ io.Writer       = (*Buffer)(nil)
)
