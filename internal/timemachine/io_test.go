package timemachine

import (
	"bytes"
	"testing"
	"testing/iotest"
)

func TestBufferedReadSeeker(t *testing.T) {
	content := bytes.Repeat([]byte("1234567890"), 10e3)
	reader := newBufferedReadSeeker(bytes.NewReader(content), 999)
	iotest.TestReader(reader, content)
}
