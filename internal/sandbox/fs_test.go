package sandbox_test

import (
	"context"
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"golang.org/x/time/rate"
)

func throttledSandboxFS(path string) sandbox.Option {
	const (
		throughput = 2 * 1024 * 1024
		burst      = throughput / 2
	)
	return sandbox.Mount("/",
		sandbox.ThrottleFS(sandbox.PathFS(path),
			rate.NewLimiter(throughput, burst),
			rate.NewLimiter(throughput, burst),
		),
	)
}

func TestSystemFS(t *testing.T) {
	t.Run("fstest", func(t *testing.T) {
		ctx := context.Background()
		sys := sandbox.New(throttledSandboxFS("testdata/fstest"))
		defer sys.Close(ctx)
		testFS(t, sys.FS())
	})
}

func testFS(t *testing.T, fsys fs.FS) {
	assert.OK(t, fstest.TestFS(fsys,
		"answer",
		"empty",
		"message",
		"tmp/one",
		"tmp/two",
		"tmp/three",
	))
}
