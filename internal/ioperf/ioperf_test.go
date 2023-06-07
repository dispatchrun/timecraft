package ioperf_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/ioperf"
)

func TestDoubleBufferedReader(t *testing.T) {
	b := new(bytes.Buffer)
	b.Grow(1e6)

	for i := 0; i < 100e3; i++ {
		b.WriteString("1234567890")
	}

	r := ioperf.NewDoubleBufferedReader(bytes.NewReader(b.Bytes()))
	defer r.Close()

	w := new(bytes.Buffer)
	w.Grow(b.Len())

	n, err := io.Copy(w, r)
	assert.OK(t, err)
	assert.Equal(t, int(n), b.Len())
	assert.True(t, bytes.Equal(w.Bytes(), b.Bytes()))
}
