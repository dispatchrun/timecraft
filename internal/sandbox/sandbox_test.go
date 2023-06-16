package sandbox_test

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

func TestSandboxFS(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New(sandbox.FS(os.DirFS("testdata")))
	defer sys.Close(ctx)
	assert.OK(t, fstest.TestFS(sys.FS(),
		"answer",
		"empty",
		"message",
		"tmp/one",
		"tmp/two",
		"tmp/three",
	))
}
