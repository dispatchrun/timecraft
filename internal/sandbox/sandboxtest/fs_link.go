package sandboxtest

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestLink = fsTestSuite{
	"creating a link to a file that does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Link(fsys, "nope", "link")
		assert.Error(t, err, sandbox.ENOENT)
	},

	"creating a link on a closed file errors with EBADF": func(t *testing.T, fsys sandbox.FileSystem) {
		f, err := sandbox.Create(fsys, "test", 0600)
		assert.OK(t, err)
		assert.OK(t, f.Close())

		err = f.Link("f1", f, "f2", sandbox.AT_SYMLINK_NOFOLLOW)
		assert.Error(t, err, sandbox.EBADF)
	},

	"creating a link to a directory errors with EPERM": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Mkdir(fsys, "test", 0700)
		assert.OK(t, err)

		err = sandbox.Link(fsys, "test", "link")
		assert.Error(t, err, sandbox.EPERM)
	},

	"creating a link to symlink is allowed": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.Symlink(fsys, "test", "link-1")
		assert.OK(t, err)

		err = sandbox.Link(fsys, "link-1", "link-2")
		assert.OK(t, err)

		s, err := sandbox.Readlink(fsys, "link-2")
		assert.OK(t, err)
		assert.Equal(t, s, "test")
	},

	"mutating the content of a file affects all links": func(t *testing.T, fsys sandbox.FileSystem) {
		assert.OK(t, sandbox.WriteFile(fsys, "test", []byte("hello"), 0600))
		assert.OK(t, sandbox.Link(fsys, "test", "link-1"))
		assert.OK(t, sandbox.Link(fsys, "test", "link-2"))
		assert.OK(t, sandbox.Link(fsys, "test", "link-3"))

		f, err := fsys.Open("link-2", sandbox.O_APPEND|sandbox.O_WRONLY, 0)
		assert.OK(t, err)
		defer f.Close()

		_, err = f.Writev([][]byte{[]byte(", world!")})
		assert.OK(t, err)

		for _, name := range []string{"test", "link-1", "link-2", "link-3"} {
			b, err := sandbox.ReadFile(fsys, name, 0)
			assert.OK(t, err)
			assert.Equal(t, string(b), "hello, world!")
		}
	},
}
