package ocifs_test

import (
	"io/fs"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/ocifs"
)

func TestOpaque(t *testing.T) {
	fsys := ocifs.Opaque()

	d, err := sandbox.OpenRoot(fsys)
	assert.OK(t, err)
	defer d.Close()

	b := make([]byte, 256)
	n, err := d.ReadDirent(b)
	assert.OK(t, err)
	b = b[:n]

	n, typ, _, _, name, err := sandbox.ReadDirent(b)
	assert.OK(t, err)
	assert.Equal(t, typ, fs.ModeDir)
	assert.Equal(t, string(name), ".")
	b = b[n:]

	n, typ, _, _, name, err = sandbox.ReadDirent(b)
	assert.OK(t, err)
	assert.Equal(t, typ, fs.ModeDir)
	assert.Equal(t, string(name), "..")
	b = b[n:]

	assert.Equal(t, len(b), 0)
}
