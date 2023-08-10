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

		_, err = f.Stat("", 0)
		assert.Error(t, err, sandbox.EBADF)
	},

	"stat contains the size of the file": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "test", []byte("hello"), 0600))
		s, err := sandbox.Stat(fsys, "test")
		assert.OK(t, err)
		assert.Equal(t, s.Mode.Type(), 0)
		assert.Equal(t, s.Size, 5)
	},

	"stat of a symlink provides information about the link target": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "test", []byte("hello"), 0600))
		assert.OK(t, sandbox.Symlink(fsys, "test", "link"))
		s, err := sandbox.Stat(fsys, "link")
		assert.OK(t, err)
		assert.Equal(t, s.Mode.Type(), 0)
		assert.Equal(t, s.Size, 5)
	},

	"stat of a non-existing directory returns ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.Mkdir(fsys, "test", 0700))

		_, err := sandbox.Stat(fsys, "test/a/b")
		assert.Error(t, err, sandbox.ENOENT) // "a" does not exist
	},

	"stat of a path which contains a file instead of a directory returns ENOTDIR": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.Mkdir(fsys, "test", 0700))
		assert.OK(t, sandbox.WriteFile(fsys, "test/a", []byte("123"), 0600))

		_, err := sandbox.Stat(fsys, "test/a/b")
		assert.Error(t, err, sandbox.ENOTDIR) // "a" is a file, not a directory
	},
}
