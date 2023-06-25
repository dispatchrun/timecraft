package timemachine

import (
	"bytes"
	"testing"
	"testing/iotest"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
)

func TestBufferedReadSeeker(t *testing.T) {
	content := bytes.Repeat([]byte("1234567890"), 10e3)
	reader := newBufferedReadSeeker(bytes.NewReader(content), 999)
	assert.OK(t, iotest.TestReader(reader, content))
}
