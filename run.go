package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/timecraft"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

const runUsage = `
Usage:	timecraft run [options] [--] <module> [args...]

Options:
   -c, --config path              Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -D, --dial addr                Expose a socket connected to the specified address
       --dir dir                  Expose a directory to the guest module
   -e, --env name=value           Pass an environment variable to the guest module
       --fly-blind                Disable recording of the guest module execution
   -h, --help                     Show this usage information
   -L, --listen addr              Expose a socket listening on the specified address
       --restrict                 Do not automatically expose the environment and root directory to the guest module
   -S, --sockets extension        Enable a sockets extension, one of none, auto, path_open, wasmedgev1, wasmedgev2 (default to auto)
       --record-batch-size size   Number of records written per batch (default to 4096)
       --record-compression type  Compression to use when writing records, either snappy or zstd (default to zstd)
   -T, --trace                    Enable strace-like logging of host function calls
`

func run(ctx context.Context, args []string) error {
	var (
		envs        stringList
		listens     stringList
		dials       stringList
		dirs        stringList
		batchSize   = human.Count(4096)
		compression = compression("zstd")
		sockets     = sockets("auto")
		flyBlind    = false
		restrict    = false
		trace       = false
	)

	flagSet := newFlagSet("timecraft run", runUsage)
	customVar(flagSet, &envs, "e", "env")
	customVar(flagSet, &listens, "L", "listen")
	customVar(flagSet, &dials, "D", "dial")
	customVar(flagSet, &dirs, "dir")
	customVar(flagSet, &sockets, "S", "sockets")
	boolVar(flagSet, &trace, "T", "trace")
	boolVar(flagSet, &flyBlind, "fly-blind")
	boolVar(flagSet, &restrict, "restrict")
	customVar(flagSet, &batchSize, "record-batch-size")
	customVar(flagSet, &compression, "record-compression")

	if err := flagSet.Parse(args); err != nil {
		return err
	}
	args = flagSet.Args()
	if len(args) == 0 {
		return errors.New(`missing "--" separator before the module path`)
	}

	var wasmPath string
	wasmPath, args = args[0], args[1:]

	if !restrict {
		envs = append(os.Environ(), envs...)
		dirs = append([]string{"/"}, dirs...)
	}

	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	config, err := timecraft.LoadConfig()
	if err != nil {
		return err
	}
	registry, err := timecraft.CreateRegistry(config)
	if err != nil {
		return err
	}
	runtime, err := timecraft.NewRuntime(ctx, config)
	if err != nil {
		return err
	}
	defer runtime.Close(ctx)

	runner := timecraft.NewRunner(ctx, registry, runtime)

	moduleSpec := timecraft.ModuleSpec{
		Path:    wasmPath,
		Args:    args,
		Env:     envs,
		Dirs:    dirs,
		Dials:   dials,
		Listens: listens,
		Sockets: string(sockets),
		Stdin:   int(os.Stdin.Fd()),
		Stdout:  int(os.Stdout.Fd()),
		Stderr:  int(os.Stderr.Fd()),
	}
	if trace {
		moduleSpec.Trace = os.Stderr
	}

	var logSpec *timecraft.LogSpec
	if !flyBlind {
		logSpec = &timecraft.LogSpec{
			ProcessID: uuid.New(),
			StartTime: time.Now(),
			BatchSize: int(batchSize),
		}

		switch compression {
		case "snappy":
			logSpec.Compression = timemachine.Snappy
		case "zstd":
			logSpec.Compression = timemachine.Zstd
		case "none", "":
			logSpec.Compression = timemachine.Uncompressed
		default:
			return fmt.Errorf("invalid compression type %q", compression)
		}

		fmt.Fprintf(os.Stderr, "%s\n", logSpec.ProcessID)
	}

	preparedModule, err := runner.Prepare(moduleSpec, logSpec)
	if err != nil {
		return err
	}
	defer preparedModule.Close()

	return runner.RunModule(preparedModule)
}
