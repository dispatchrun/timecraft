package sandboxtest

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestRename = fsTestSuite{
	"renaming a closed file errors with EBADF": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.WriteFile(fsys, "test", []byte("hello"), 0600)
		assert.OK(t, err)

		d, err := sandbox.OpenRoot(fsys)
		assert.OK(t, err)
		assert.OK(t, d.Close())

		err = d.Rename("test", d, "nope")
		assert.Error(t, err, sandbox.EBADF)

		b, err := sandbox.ReadFile(fsys, "test", 0)
		assert.OK(t, err)
		assert.Equal(t, string(b), "hello")
	},

	"renaming a file that does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Rename(fsys, "old", "new")
		assert.Error(t, err, sandbox.ENOENT)
	},

	"renaming a file to a location where a file already exists replaces it": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "one", []byte("1"), 0600))
		assert.OK(t, sandbox.WriteFile(fsys, "two", []byte("2"), 0600))
		assert.OK(t, sandbox.Rename(fsys, "two", "one"))

		b, err := sandbox.ReadFile(fsys, "one", 0)
		assert.OK(t, err)
		assert.Equal(t, string(b), "2")

		_, err = sandbox.ReadFile(fsys, "two", 0)
		assert.Error(t, err, sandbox.ENOENT)
	},

	"renaming a file to a location where a symlink already exists replaces it": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.Symlink(fsys, "test", "one"))
		assert.OK(t, sandbox.WriteFile(fsys, "two", []byte("2"), 0600))
		assert.OK(t, sandbox.Rename(fsys, "two", "one"))

		b, err := sandbox.ReadFile(fsys, "one", 0)
		assert.OK(t, err)
		assert.Equal(t, string(b), "2")

		_, err = sandbox.ReadFile(fsys, "two", 0)
		assert.Error(t, err, sandbox.ENOENT)
	},

	"renaming a file to a location where a directory exists errors with EISDIR": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.Mkdir(fsys, "one", 0700))
		assert.OK(t, sandbox.WriteFile(fsys, "two", []byte("2"), 0600))

		err := sandbox.Rename(fsys, "two", "one")
		assert.Error(t, err, sandbox.EISDIR)
	},
}
