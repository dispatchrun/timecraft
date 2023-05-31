package main

import (
	"context"
)

const unknownCommand = `timecraft %s: unknown command
For a list of commands available, run 'timecraft help.'
`

func unknown(ctx context.Context, cmd string) error {
	return usageError(unknownCommand, cmd)
}
