package cmd

import (
	"context"
	"fmt"
)

const helpUsage = `
Usage:	timecraft <command> [options]

Runtime Commands:
   run      Run a WebAssembly module, and optionally trace execution
   replay   Replay a recorded trace of execution

Debugging Commands:
   profile  Generate performance profile from execution records

Other Commands:
   help     Show usage information about timecraft commands
   version  Show the timecraft version information

For a description of each command, run 'timecraft help <command>'.`

func help(ctx context.Context, args []string) error {
	flagSet := newFlagSet("timecraft help", helpUsage)
	parseFlags(flagSet, args)

	var cmd string
	var msg string

	if args = flagSet.Args(); len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "get":
		msg = getUsage
	case "help", "":
		msg = helpUsage
	case "profile":
		msg = profileUsage
	case "run":
		msg = runUsage
	case "replay":
		msg = replayUsage
	case "version":
		msg = versionUsage
	default:
		fmt.Printf("timecraft help %s: unknown command\n", cmd)
		return ExitCode(1)
	}

	fmt.Println(msg)
	return nil
}
