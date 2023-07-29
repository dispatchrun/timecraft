package sandboxtest

import (
	"io/fs"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestLstat = fsTestSuite{
	"lstat on a file that does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		_, err := sandbox.Lstat(fsys, "nope")
		assert.Error(t, err, sandbox.ENOENT)
	},

	"lstat on a closed file errors with EBADF": func(t *testing.T, fsys sandbox.FileSystem) {
		f, err := sandbox.Create(fsys, "test", 0600)
		assert.OK(t, err)
		assert.OK(t, f.Close())

		_, err = f.Lstat("link")
		assert.Error(t, err, sandbox.EBADF)
	},

	"lstat contains the size of the file": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "test", []byte("hello"), 0600))
		s, err := sandbox.Lstat(fsys, "test")
		assert.OK(t, err)
		assert.Equal(t, s.Mode().Type(), 0)
		assert.Equal(t, s.Size(), 5)
	},

	"lstat of a symlink provides information about the link itself": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "test", []byte("hello"), 0600))
		assert.OK(t, sandbox.Symlink(fsys, "test", "link"))
		s, err := sandbox.Lstat(fsys, "link")
		assert.OK(t, err)
		assert.Equal(t, s.Mode().Type(), fs.ModeSymlink)
		assert.Equal(t, s.Size(), 4)
	},
}
