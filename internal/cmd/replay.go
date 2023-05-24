package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
)

func replayUsage() {
	fmt.Print(`timecraft replay - Replay a recorded trace of execution

USAGE:
   timecraft replay [OPTIONS]... <LOG> <MODULE>

ARGS:
   <LOG>
      The path of the log that contains the recorded trace of execution

   <MODULE>
      The path of the WebAssembly module to run the replay against

OPTIONS:
   -h, --help
      Show this usage information
`)
}

func replay(args []string) error {
	flagSet := flag.NewFlagSet("timecraft run", flag.ExitOnError)
	flagSet.Usage = replayUsage

	flagSet.Parse(args)

	args = flagSet.Args()
	if len(args) != 2 {
		replayUsage()
		os.Exit(1)
	}
	logPath := args[0]
	wasmPath := args[1]

	logFile, err := os.Open(logPath)
	if err != nil {
		return err
	}
	defer logFile.Close()

	logReader := timemachine.NewLogReader(logFile)
	defer logReader.Close()

	logHeader, _, err := logReader.ReadLogHeader()
	if err != nil {
		return fmt.Errorf("cannot read header from log %q: %w", logPath, err)
	}

	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("could not read WASM file '%s': %w", wasmPath, err)
	}

	switch imageHash := logHeader.Process.Image; imageHash.Algorithm {
	case "sha256":
		if hash := timemachine.SHA256(wasmCode); hash != imageHash {
			return fmt.Errorf("image hash mismatch for %q: got %s, expect %s", wasmPath, hash.Digest, imageHash.Digest)
		}
	default:
		return fmt.Errorf("unsupported process image hash algorithm: %q", imageHash.Algorithm)
	}

	ctx := context.Background()
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
