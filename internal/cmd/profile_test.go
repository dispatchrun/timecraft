package cmd_test

import (
	"context"

	"github.com/stealthrocket/timecraft/internal/cmd"
)

func ExampleRoot_profileMissingID() {
	ctx := context.Background()

	FAIL(cmd.Root(ctx, "profile"))
	// Output:
	// ERR: timecraft profile: expected exactly one process id as argument
}

func ExampleRoot_profileTooManyArgs() {
	ctx := context.Background()

	FAIL(cmd.Root(ctx, "profile", "1", "2", "3"))
	// Output:
	// ERR: timecraft profile: expected exactly one process id as argument
}

func ExampleRoot_profileInvalidID() {
	ctx := context.Background()

	FAIL(cmd.Root(ctx, "profile", "1234567890"))
	// Output:
	// ERR: timecraft profile: malformed process id passed as argument (not a UUID)
}

func ExampleRoot_profileUnknownID() {
	ctx := context.Background()

	FAIL(cmd.Root(ctx, "profile", "b0f4dac5-9855-4cde-89fd-ebd3713c2249"))
	// Output:
	// ERR: timecraft profile: process has no records: b0f4dac5-9855-4cde-89fd-ebd3713c2249
}
