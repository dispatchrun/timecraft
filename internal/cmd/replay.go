package cmd

import (
	"context"
	"flag"
	"fmt"

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
      (default to ~/.timecraft/data)
`)
}

func replay(ctx context.Context, args []string) error {
	var (
		store string
	)

	flagSet := flag.NewFlagSet("timecraft run", flag.ExitOnError)
	flagSet.Usage = replayUsage
	flagSet.StringVar(&store, "store", "~/.timecraft/data", "")
	flagSet.Parse(args)

	args = flagSet.Args()
	if len(args) != 1 {
		replayUsage()
		return ExitCode(1)
	}

	processID, err := timemachine.ParseHash(args[0])
	if err != nil {
		replayUsage()
		return err
	}

	timestore, err := openStore(store)
	if err != nil {
		return err
	}

	logSegment, err := timestore.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return err
	}
	defer logSegment.Close()

	logReader := timemachine.NewLogReader(logSegment)
	defer logReader.Close()

	logHeader, err := logReader.ReadLogHeader()
	if err != nil {
		return fmt.Errorf("cannot read header from log of %s: %w", processID, err)
	}

	wasmCode, err := timestore.ReadModule(ctx, logHeader.Process.Image)
	if err != nil {
		return fmt.Errorf("cannot read module byte code of %s: %w", processID, err)
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	wasmModule, err := runtime.CompileModule(ctx, wasmCode)
	if err != nil {
		return err
	}
	defer wasmModule.Close(ctx)

	var functions timemachine.FunctionIndex
	importedFunctions := wasmModule.ImportedFunctions()
	for _, f := range importedFunctions {
		moduleName, functionName, isImport := f.Import()
		if !isImport {
			continue
		}
		functions.Add(timemachine.Function{
			Module:      moduleName,
			Name:        functionName,
			ParamCount:  len(f.ParamTypes()),
			ResultCount: len(f.ResultTypes()),
		})
	}
	if len(logHeader.Runtime.Functions) != len(functions.Functions()) {
		return fmt.Errorf("imported functions mismatch")
	}
	for i, fn := range functions.Functions() {
		if fn != logHeader.Runtime.Functions[i] {
			return fmt.Errorf("imported functions mismatch")
		}
	}

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

	instance, err := runtime.InstantiateModule(ctx, wasmModule, wazero.NewModuleConfig())
	if err != nil {
		return err
	}
	return instance.Close(ctx)
}
