package sandboxtest

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestOpen = fsTestSuite{
	"opening a file that does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		_, err := sandbox.Open(fsys, "nope")
		assert.Error(t, err, sandbox.ENOENT)
	},

	"existing files can be opened": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "test", []byte("hello"), 0600))
		f, err := sandbox.Open(fsys, "test")
		assert.OK(t, err)
		assert.OK(t, f.Close())
	},

	"existing symlinks cannot be opened": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.Symlink(fsys, "test", "link"))
		_, err := fsys.Open("link", sandbox.O_NOFOLLOW, 0)
		assert.Error(t, err, sandbox.ELOOP)
	},

	"symlinks can be read to obtain their target": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.Symlink(fsys, "test", "link"))
		s, err := sandbox.Readlink(fsys, "link")
		assert.OK(t, err)
		assert.Equal(t, s, "test")
	},
}
