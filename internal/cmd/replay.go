package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/stealthrocket/wasi-go"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
)

const replayUsage = `
Usage:	timecraft replay [options] <process id>

Options:
   -h, --help           Show this usage information
   -r, --registry path  Path to the timecraft registry (default to ~/.timecraft)
   -T, --trace          Enable strace-like logging of host function calls
`

func replay(ctx context.Context, args []string) error {
	var (
		registryPath = "~/.timecraft"
		trace        = false
	)

	flagSet := newFlagSet("timecraft replay", replayUsage)
	stringVar(flagSet, &registryPath, "r", "registry")
	boolVar(flagSet, &trace, "T", "trace")
	flagSet.Parse(args)

	args = flagSet.Args()
	if len(args) != 1 {
		return errors.New(`expected exactly one process id as argument`)
	}

	processID, err := uuid.Parse(args[0])
	if err != nil {
		return errors.New(`malformed process id passed as argument (not a UUID)`)
	}

	registry, err := openRegistry(registryPath)
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
	config, err := registry.LookupConfig(ctx, process.Config.Digest)
	if err != nil {
		return err
	}
	module, err := registry.LookupModule(ctx, config.Modules[0].Digest)
	if err != nil {
		return err
	}

	logSegment, err := registry.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return err
	}
	defer logSegment.Close()

	logReader := timemachine.NewLogReader(logSegment, manifest.StartTime)
	defer logReader.Close()

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiledModule, err := runtime.CompileModule(ctx, module.Code)
	if err != nil {
		return err
	}
	defer compiledModule.Close(ctx)

	records := timemachine.NewLogRecordReader(logReader)

	replay := wasicall.NewReplay(records)
	defer replay.Close(ctx)

	fallback := wasicall.NewObserver(nil, func(ctx context.Context, s wasicall.Syscall) {
		panic(fmt.Sprintf("system call made after log EOF: %s", s.ID()))
	}, nil)
	var system wasi.System = wasicall.NewFallbackSystem(replay, fallback)
	if trace {
		system = &wasi.Tracer{Writer: os.Stderr, System: system}
	}

	// TODO: need to figure this out dynamically:
	hostModule := wasi_snapshot_preview1.NewHostModule(wasi_snapshot_preview1.WasmEdgeV2)
	hostModuleInstance := wazergo.MustInstantiate(ctx, runtime, hostModule, wasi_snapshot_preview1.WithWASI(system))
	ctx = wazergo.WithModuleInstance(ctx, hostModuleInstance)

	return exec(ctx, runtime, compiledModule)
}
