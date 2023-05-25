package cmd

import (
	"context"
	"flag"
	"fmt"

	"github.com/google/uuid"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
)

func replayUsage() {
	fmt.Print(`timecraft replay - Replay a recorded trace of execution

USAGE:
   timecraft replay [OPTIONS]... <ID>

ARGS:
   <LOG>
      The path of the log that contains the recorded trace of execution

   <MODULE>
      The path of the WebAssembly module to run the replay against

OPTIONS:
   -h, --help
      Show this usage information

   --store <PATH>
      Path to the directory where the timecraft object store is available
      (default to ~/.timecraft)
`)
}

func replay(ctx context.Context, args []string) error {
	var (
		store string
	)

	flagSet := flag.NewFlagSet("timecraft run", flag.ExitOnError)
	flagSet.Usage = replayUsage
	flagSet.StringVar(&store, "store", "~/.timecraft", "")
	flagSet.Parse(args)

	args = flagSet.Args()
	if len(args) != 1 {
		replayUsage()
		return ExitCode(1)
	}

	processID, err := uuid.Parse(args[0])
	if err != nil {
		replayUsage()
		return err
	}

	timestore, err := openStore(store)
	if err != nil {
		return err
	}

	manifest, err := timestore.LookupLogManifest(ctx, processID)
	if err != nil {
		return err
	}
	process, err := timestore.LookupProcess(ctx, manifest.Process.Digest)
	if err != nil {
		return err
	}
	config, err := timestore.LookupConfig(ctx, process.Config.Digest)
	if err != nil {
		return err
	}
	module, err := timestore.LookupModule(ctx, config.Modules[0].Digest)
	if err != nil {
		return err
	}

	logSegment, err := timestore.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return err
	}
	defer logSegment.Close()

	logReader := timemachine.NewLogReader(logSegment, manifest.StartTime)
	defer logReader.Close()

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiledModule, err := runtime.CompileModule(ctx, module)
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
	system := wasicall.NewFallbackSystem(replay, fallback)

	// TODO: need to figure this out dynamically:
	hostModule := wasi_snapshot_preview1.NewHostModule(wasi_snapshot_preview1.WasmEdgeV2)
	hostModuleInstance := wazergo.MustInstantiate(ctx, runtime, hostModule, wasi_snapshot_preview1.WithWASI(system))
	ctx = wazergo.WithModuleInstance(ctx, hostModuleInstance)

	guestModuleInstance, err := runtime.InstantiateModule(ctx, compiledModule, wazero.NewModuleConfig().
		WithStartFunctions())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		_, err := guestModuleInstance.ExportedFunction("_start").Call(ctx)
		cancel(err)
	}()
	<-ctx.Done()
	return guestModuleInstance.Close(ctx)
}
