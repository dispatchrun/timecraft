package main

import (
	"context"
	"errors"
	"os"

	"github.com/stealthrocket/timecraft/internal/timecraft"
)

const replayUsage = `
Usage:	timecraft replay [options] <process id>

Options:
   -c, --config path  Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -h, --help         Show this usage information
   -q, --quiet        Do not output the recording of stdout/stderr during the replay
   -T, --trace        Enable strace-like logging of host function calls
`

func replay(ctx context.Context, args []string) error {
	var (
		quiet = false
		trace = false
	)

	flagSet := newFlagSet("timecraft replay", replayUsage)
	boolVar(flagSet, &quiet, "q", "quiet")
	boolVar(flagSet, &trace, "T", "trace")

	args, err := parseFlags(flagSet, args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New(`expected exactly one process id as argument`)
	}

	processID, err := parseProcessID(args[0])
	if err != nil {
		return err
	}
	config, err := timecraft.LoadConfig()
	if err != nil {
		return err
	}
	registry, err := timecraft.OpenRegistry(config)
	if err != nil {
		return err
	}
	runtime, ctx, err := timecraft.NewRuntime(ctx, config)
	if err != nil {
		return err
	}
	defer runtime.Close(ctx)

	replay := timecraft.NewReplay(registry, runtime, processID)
	if !quiet {
		replay.SetStdout(os.Stdout)
		replay.SetStderr(os.Stderr)
	}
	if trace {
		replay.SetTrace(os.Stderr)
	}
	return replay.Replay(ctx)
}
