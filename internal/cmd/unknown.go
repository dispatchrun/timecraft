package cmd

import (
	"context"
	"fmt"
)

const unknownCommand = `timecraft %s: unknown command
For a list of commands available, run 'timecraft help'
`

func unknown(ctx context.Context, cmd string) error {
	fmt.Printf(unknownCommand, cmd)
	return ExitCode(1)
}
