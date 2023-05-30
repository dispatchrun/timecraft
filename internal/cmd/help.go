package cmd

import (
	"context"
	"fmt"
)

const helpUsage = `
Usage:	timecraft <command> [options]

Registry Commands:
   describe  Show detailed information about specific resources
   export    Export resources to local files
   get       Display resources from the time machine registry

Runtime Commands:
   run       Run a WebAssembly module, and optionally trace execution
   replay    Replay a recorded trace of execution

Debugging Commands:
   profile   Generate performance profile from execution records

Other Commands:
   config    View or edit the timecraft configuration
   help      Show usage information about timecraft commands
   version   Show the timecraft version information

Global Options:
   -c, --config  Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -h, --help    Show usage information

For a description of each command, run 'timecraft help <command>'.`

func help(ctx context.Context, args []string) error {
	flagSet := newFlagSet("timecraft help", helpUsage)
	args = parseFlags(flagSet, args)
	if len(args) == 0 {
		args = []string{"help"}
	}

	for i, cmd := range args {
		var msg string

		if i != 0 {
			fmt.Println("---")
		}

		switch cmd {
		case "config":
			msg = configUsage
		case "describe":
			msg = describeUsage
		case "export":
			msg = exportUsage
		case "get":
			msg = getUsage
		case "help":
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
	}
	return nil
}
