package cmd_test

import (
	"context"

	"github.com/stealthrocket/timecraft/internal/cmd"
)

func ExampleRoot_version() {
	ctx := context.Background()

	PASS(cmd.Root(ctx, "version"))
	// Output:
	// timecraft devel
}
