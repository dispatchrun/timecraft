package sandbox_test

import (
	"io/fs"
	"testing"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/sandboxtest"
)

func TestDirFS(t *testing.T) {
	t.Run("fs.FS", func(t *testing.T) {
		sandboxtest.TestFS(t, func(t *testing.T, path string) fs.FS {
			return sandbox.FS(sandbox.DirFS(path))
		})
	})

	sandboxtest.TestFileSystem(t, func(t *testing.T) sandbox.FileSystem {
		return sandbox.DirFS(t.TempDir())
	})
}
