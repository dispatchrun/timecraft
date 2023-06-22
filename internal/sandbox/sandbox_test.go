package sandbox_test

import (
	"context"
	"io"
	"os"
	"testing"
	"testing/fstest"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/wasitest"
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

func TestSandboxSystem(t *testing.T) {
	wasitest.TestSystem(t, func(config wasitest.TestConfig) (wasi.System, error) {
		options := []sandbox.Option{
			sandbox.Args(config.Args...),
			sandbox.Environ(config.Environ...),
			sandbox.Rand(config.Rand),
			sandbox.Time(config.Now),
		}

		if config.RootFS != "" {
			options = append(options, sandbox.FS(os.DirFS(config.RootFS)))
		}

		sys := sandbox.New(options...)

		stdin, stdout, stderr := sys.Stdin(), sys.Stdout(), sys.Stderr()
		go copyAndClose(stdin, config.Stdin)
		go copyAndClose(config.Stdout, stdout)
		go copyAndClose(config.Stderr, stderr)

		return sys, nil
		//return wasi.Trace(os.Stderr, sys), nil
	})
}

func copyAndClose(w io.WriteCloser, r io.ReadCloser) {
	if w != nil {
		defer w.Close()
	}
	if r != nil {
		defer r.Close()
	}
	if w != nil && r != nil {
		_, _ = io.Copy(w, r)
	}
}
