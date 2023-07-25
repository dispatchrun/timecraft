package sandbox_test

import (
	"context"
	"testing"
	"testing/fstest"

	"golang.org/x/time/rate"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

func rootFS(path string) sandbox.Option {
	const (
		throughput = 2 * 1024 * 1024
		burst      = throughput / 2
	)
	return sandbox.Mount("/",
		sandbox.ThrottleFS(sandbox.DirFS(path),
			rate.NewLimiter(throughput, burst),
			rate.NewLimiter(throughput, burst),
		),
	)
}

func TestSandboxFS(t *testing.T) {
	ctx := context.Background()
	sys := sandbox.New(rootFS("testdata"))
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
