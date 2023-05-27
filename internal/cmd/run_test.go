package cmd_test

import (
	"context"

	"github.com/stealthrocket/timecraft/internal/cmd"
)

func ExampleRoot_runExitZero() {
	ctx := context.Background()

	OK(cmd.Root(ctx, "run", "../../testdata/go/sleep.wasm", "10ms"))
	// Output: sleeping for 10ms
}

func ExampleRoot_runContextCanceled() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	OK(cmd.Root(ctx, "run", "../../testdata/go/sleep.wasm", "10s"))
	// Output:
}
