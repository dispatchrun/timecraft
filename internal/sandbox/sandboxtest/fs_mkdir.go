package sandboxtest

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestMkdir = fsTestSuite{
	"creating directories at a location which does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Mkdir(fsys, "sub/dir", 0700)
		assert.Error(t, err, sandbox.ENOENT)
	},

	"creating directories on a closed file errors with EBADF": func(t *testing.T, fsys sandbox.FileSystem) {
		f, err := sandbox.OpenRoot(fsys)
		assert.OK(t, err)
		assert.OK(t, f.Close())

		err = f.Mkdir("test", 0700)
		assert.Error(t, err, sandbox.EBADF)
	},

	"creating a directory at a location where a directory already exists errors with EEXIST": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Mkdir(fsys, "tmp", 0700)
		assert.OK(t, err)

		err = sandbox.Mkdir(fsys, "tmp", 0700)
		assert.Error(t, err, sandbox.EEXIST)
	},

	"creating a directory at a location where a file already exists errors with EEXIST": func(t *testing.T, fsys sandbox.FileSystem) {
		f, err := sandbox.Create(fsys, "test", 0600)
		assert.OK(t, err)
		assert.OK(t, f.Close())

		err = sandbox.Mkdir(fsys, "test", 0700)
		assert.Error(t, err, sandbox.EEXIST)
	},

	"creating a directory at a location where a symlink already exists errors with EEXIST": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Symlink(fsys, "test", "link")
		assert.OK(t, err)

		err = sandbox.Mkdir(fsys, "link", 0700)
		assert.Error(t, err, sandbox.EEXIST)
	},

	"creating a path ignores directories that already exist": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.MkdirAll(fsys, "path/to/", 0700)
		assert.OK(t, err)

		err = sandbox.MkdirAll(fsys, "path/to/file", 0700)
		assert.OK(t, err)

		d, err := sandbox.OpenDir(fsys, "path/to/file/")
		assert.OK(t, err)
		assert.OK(t, d.Close())
	},
}
