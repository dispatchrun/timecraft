package stdio_test

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/debug/stdio"
)

func TestBufferWrite(t *testing.T) {
	const text = `hello world!
second line
third line
a
b
the last line has no return`

	b := new(bytes.Buffer)
	w := stdio.NewBuffer(b)
	r := strings.NewReader(text)

	n, err := io.Copy(w, iotest.OneByteReader(r))
	assert.OK(t, err)
	assert.Equal(t, int(n), len(text))

	assert.OK(t, w.Flush())
	assert.Equal(t, b.String(), text)
}
