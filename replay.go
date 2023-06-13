package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
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
	config, err := loadConfig()
	if err != nil {
		return err
	}
	registry, err := config.openRegistry()
	if err != nil {
		return err
	}

	manifest, err := registry.LookupLogManifest(ctx, processID)
	if err != nil {
		return err
	}
	process, err := registry.LookupProcess(ctx, manifest.Process.Digest)
	if err != nil {
		return err
	}
	processConfig, err := registry.LookupConfig(ctx, process.Config.Digest)
	if err != nil {
		return err
	}
	module, err := registry.LookupModule(ctx, processConfig.Modules[0].Digest)
	if err != nil {
		return err
	}

	logSegment, err := registry.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return err
	}
	defer logSegment.Close()

	logReader := timemachine.NewLogReader(logSegment, manifest)
	defer logReader.Close()

	runtime := config.newRuntime(ctx)
	defer runtime.Close(ctx)

	compiledModule, err := runtime.CompileModule(ctx, module.Code)
	if err != nil {
		return err
	}
	defer compiledModule.Close(ctx)

	records := timemachine.NewLogRecordReader(logReader)

	replay := wasicall.NewReplay(records)
	defer replay.Close(ctx)

	if !quiet {
		replay.Stdout = os.Stdout
		replay.Stderr = os.Stderr
	}

	fallback := wasicall.NewObserver(nil, func(ctx context.Context, s wasicall.Syscall) {
		panic(fmt.Sprintf("system call made after log EOF: %s", s.ID()))
	}, nil)

	system := wasi.System(wasicall.NewFallbackSystem(replay, fallback))
	if trace {
		system = wasi.Trace(os.Stderr, system)
	}

	hostModule := wasi_snapshot_preview1.NewHostModule(imports.DetectExtensions(compiledModule)...)
	hostModuleInstance := wazergo.MustInstantiate(ctx, runtime, hostModule, wasi_snapshot_preview1.WithWASI(system))
	ctx = wazergo.WithModuleInstance(ctx, hostModuleInstance)

	return instantiate(ctx, runtime, compiledModule)
}
