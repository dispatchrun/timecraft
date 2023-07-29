package sandboxtest

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestRmdir = fsTestSuite{
	"removing a closed directory errors with EBADF": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.MkdirAll(fsys, "a/b/c", 0700)
		assert.OK(t, err)

		f, err := sandbox.OpenDir(fsys, "a/b")
		assert.OK(t, err)
		assert.OK(t, f.Close())

		err = f.Rmdir("c")
		assert.Error(t, err, sandbox.EBADF)
	},

	"removing a directory at a location where there is a file errors with ENOTDIR": func(t *testing.T, fsys sandbox.FileSystem) {
		f, err := sandbox.Create(fsys, "test", 0600)
		assert.OK(t, err)
		assert.OK(t, f.Close())

		err = sandbox.Rmdir(fsys, "test")
		assert.Error(t, err, sandbox.ENOTDIR)
	},

	"removing a directory at a location where there is a symlink errors with ENOTDIR": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Symlink(fsys, "test", "link")
		assert.OK(t, err)

		err = sandbox.Rmdir(fsys, "link")
		assert.Error(t, err, sandbox.ENOTDIR)
	},

	"removing a directory at a location that does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Rmdir(fsys, "nope")
		assert.Error(t, err, sandbox.ENOENT)
	},

	"removing a directory that is not empty errors with ENOTEMPTY": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.MkdirAll(fsys, "dir-1/dir-2/dir-3", 0700)
		assert.OK(t, err)

		err = sandbox.Rmdir(fsys, "dir-1/dir-2")
		assert.Error(t, err, sandbox.ENOTEMPTY)
	},

	"removing a directory deletes it from the file system": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Mkdir(fsys, "test", 0700)
		assert.OK(t, err)

		err = sandbox.Rmdir(fsys, "test")
		assert.OK(t, err)

		_, err = sandbox.Stat(fsys, "test")
		assert.Error(t, err, sandbox.ENOENT)
	},
}
