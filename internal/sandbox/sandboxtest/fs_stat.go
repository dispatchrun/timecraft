package sandboxtest

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestStat = fsTestSuite{
	"stat on a file that does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		_, err := sandbox.Stat(fsys, "nope")
		assert.Error(t, err, sandbox.ENOENT)
	},

	"stat on a closed file errors with EBADF": func(t *testing.T, fsys sandbox.FileSystem) {
		f, err := sandbox.Create(fsys, "test", 0600)
		assert.OK(t, err)
		assert.OK(t, f.Close())

		_, err = f.Stat()
		assert.Error(t, err, sandbox.EBADF)
	},

	"stat contains the size of the file": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "test", []byte("hello"), 0600))
		s, err := sandbox.Stat(fsys, "test")
		assert.OK(t, err)
		assert.Equal(t, s.Size(), 5)
	},

	"stat of a symlink provides information about the link target": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "test", []byte("hello"), 0600))
		assert.OK(t, sandbox.Symlink(fsys, "test", "link"))
		s, err := sandbox.Stat(fsys, "link")
		assert.OK(t, err)
		assert.Equal(t, s.Size(), 5)
	},
}
