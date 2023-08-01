package sandbox_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/sandboxtest"
)

func TestDirFS(t *testing.T) {
	t.Run("fstest", func(t *testing.T) {
		testFS(t, sandbox.FS(sandbox.DirFS("testdata/fstest")))
	})
	sandboxtest.TestFileSystem(t, func(t *testing.T) sandbox.FileSystem {
		return sandbox.DirFS(t.TempDir())
	})
}
