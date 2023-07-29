package sandboxtest

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestCreate = fsTestSuite{
	"creating a file at a location that does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		_, err := sandbox.Create(fsys, "tmp/file", 0600)
		assert.Error(t, err, sandbox.ENOENT)
	},

	"creating a file with O_EXCL at a location where a file already exists errors with EEXIST": func(t *testing.T, fsys sandbox.FileSystem) {
		f, err := sandbox.Create(fsys, "file", 0600)
		assert.OK(t, err)
		assert.OK(t, f.Close())

		_, err = fsys.Open("file", sandbox.O_EXCL|sandbox.O_CREAT|sandbox.O_TRUNC|sandbox.O_WRONLY, 0600)
		assert.Error(t, err, sandbox.EEXIST)
	},

	"creating a file at a location where a directory exists errors with EISDIR": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Mkdir(fsys, "test", 0700)
		assert.OK(t, err)

		_, err = sandbox.Create(fsys, "test", 0600)
		assert.Error(t, err, sandbox.EISDIR)
	},

	"creating a file at a location where a symlink exists is permitted": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Symlink(fsys, "test", "link")
		assert.OK(t, err)

		_, err = sandbox.Create(fsys, "link", 0600)
		assert.OK(t, err)
	},

	"files can be created on the file system": func(t *testing.T, fsys sandbox.FileSystem) {
		f, err := sandbox.Create(fsys, "file", 0600)
		assert.OK(t, err)
		assert.OK(t, f.Close())
	},
}
