package sandbox_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

func TestDirFS(t *testing.T) {
	t.Run("fstest", func(t *testing.T) {
		testFS(t, sandbox.FS(sandbox.DirFS("testdata/fstest")))
	})
}
