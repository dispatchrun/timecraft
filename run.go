package main

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/internal/chaos"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/timecraft"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go"
)

const runUsage = `
Usage:	timecraft run [options] [--] <module> [args...]

Options:
   -C, --chaotic ratio            Enable artificial fault injection when running the module (raio is a decimal value between 0 and 1)
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
		chaotic     = human.Ratio(0)
		batchSize   = human.Count(4096)
		compression = compression("zstd")
		sockets     = sockets("auto")
		flyBlind    = false
		restrict    = false
		trace       = false
		proxy       = ""
	)

	flagSet := newFlagSet("timecraft run", runUsage)
	customVar(flagSet, &envs, "e", "env")
	customVar(flagSet, &listens, "L", "listen")
	customVar(flagSet, &dials, "D", "dial")
	customVar(flagSet, &dirs, "dir")
	customVar(flagSet, &sockets, "S", "sockets")
	customVar(flagSet, &chaotic, "C", "chaotic")
	boolVar(flagSet, &trace, "T", "trace")
	boolVar(flagSet, &flyBlind, "fly-blind")
	boolVar(flagSet, &restrict, "restrict")
	customVar(flagSet, &batchSize, "record-batch-size")
	customVar(flagSet, &compression, "record-compression")
	stringVar(flagSet, &proxy, "proxy")

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

	scheduler := &timecraft.TaskScheduler{}
	defer scheduler.Close()

	serverFactory := &timecraft.ServerFactory{
		Scheduler: scheduler,
	}

	var adapter func(timecraft.ProcessID, wasi.System) wasi.System
	if chaotic > 0 {
		adapter = func(process timecraft.ProcessID, system wasi.System) wasi.System {
			seed := int64(binary.LittleEndian.Uint64(process[8:]))
			prng := rand.NewSource(seed)
			chance := float64(chaotic) / 2
			system = chaos.New(prng, system,
				chaos.Chance(chance, chaos.Error(system)),
				chaos.Chance(chance, chaos.Chunk(system)),
			)
			system = chaos.LowEntropy(system)
			system = chaos.ClockDrift(system)
			return system
		}
	}

	processManager := timecraft.NewProcessManager(ctx, registry, runtime, serverFactory, adapter)
	defer processManager.Close()

	serverFactory.ProcessManager = processManager
	scheduler.ProcessManager = processManager

	moduleSpec := timecraft.ModuleSpec{
		Path:    wasmPath,
		Args:    args,
		Env:     envs,
		Dirs:    dirs,
		Dials:   dials,
		Listens: listens,
		Sockets: string(sockets),
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		// Allow the main module to open listening sockets on the host network
		// so server applications can receive connections as if they were
		// running outside of timecraft.
		HostNetworkBinding: true,
		ProxyPath:          proxy,
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

	processID, err := processManager.Start(moduleSpec, logSpec, nil)
	if err != nil {
		return err
	}
	return processManager.Wait(processID)
}
