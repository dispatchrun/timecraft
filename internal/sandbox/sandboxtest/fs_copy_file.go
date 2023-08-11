package sandboxtest

import (
	"crypto/rand"
	"io"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

var fsTestCopyFile = fsTestSuite{
	"copying a file which does not exist errors with ENOENT": func(t *testing.T, fsys sandbox.FileSystem) {
		err := sandbox.CopyFile(fsys, "src", "dst")
		assert.Error(t, err, sandbox.ENOENT)

		_, err = sandbox.Stat(fsys, "dst")
		assert.Error(t, err, sandbox.ENOENT)
	},

	"files can be copied on a file system": func(t *testing.T, fsys sandbox.FileSystem) {
		content := make([]byte, 1e6)
		_, err := io.ReadFull(rand.Reader, content)
		assert.OK(t, err)
		assert.OK(t, sandbox.WriteFile(fsys, "src", content, 0644))
		assert.OK(t, sandbox.CopyFile(fsys, "src", "dst"))

		b, err := sandbox.ReadFile(fsys, "dst", 0)
		assert.OK(t, err)
		assert.Equal(t, string(b), string(content))
	},
}
