package sandbox_test

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/wasi-go"
)

func TestSandboxFS(t *testing.T) {
	ctx := context.Background()
	sys := &sandbox.System{
		FS: os.DirFS("testdata"),
	}
	defer sys.Close(ctx)

	rootFD, err := sys.Mount(ctx)
	assert.OK(t, err)
	assert.OK(t, fstest.TestFS(wasi.FS(ctx, sys, rootFD),
		"answer",
		"empty",
		"message",
		"tmp/one",
		"tmp/two",
		"tmp/three",
	))
}
