package sandboxtest

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestUnlink = fsTestSuite{
	"unlinking a closed file errors with EBADF": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.WriteFile(fsys, "test", []byte("hello"), 0600)
		assert.OK(t, err)

		d, err := sandbox.OpenRoot(fsys)
		assert.OK(t, err)
		assert.OK(t, d.Close())

		err = d.Unlink("test")
		assert.Error(t, err, sandbox.EBADF)

		b, err := sandbox.ReadFile(fsys, "test", 0)
		assert.OK(t, err)
		assert.Equal(t, string(b), "hello")
	},

	"unlinking a file at a location that does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Unlink(fsys, "nope")
		assert.Error(t, err, sandbox.ENOENT)
	},

	"unlinking a file deletes it from the file system": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.WriteFile(fsys, "test", []byte("123"), 0600)
		assert.OK(t, err)

		b, err := sandbox.ReadFile(fsys, "test", 0)
		assert.OK(t, err)
		assert.Equal(t, string(b), "123")

		err = sandbox.Unlink(fsys, "test")
		assert.OK(t, err)

		_, err = sandbox.ReadFile(fsys, "test", 0)
		assert.Error(t, err, sandbox.ENOENT)
	},

	"unlinking a symlink deletes it from the file system": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Symlink(fsys, "test", "link")
		assert.OK(t, err)

		s, err := sandbox.Readlink(fsys, "link")
		assert.OK(t, err)
		assert.Equal(t, s, "test")

		err = sandbox.Unlink(fsys, "link")
		assert.OK(t, err)

		_, err = sandbox.Readlink(fsys, "link")
		assert.Error(t, err, sandbox.ENOENT)
	},
}
