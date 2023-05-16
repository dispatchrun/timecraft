package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero"
)

var version = "devel"

func init() {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
}

func main() {
	if err := root(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func rootUsage() {
	fmt.Print(`timecraft - A time machine for production

USAGE:
   timecraft [OPTIONS]... <COMMAND> [ARGS]...

COMMANDS:
   run  Run a WebAssembly module

OPTIONS:
   -v, --version
      Print the version and exit

   -h, --help
      Show this usage information
`)
}

func root(args []string) (err error) {
	flagSet := flag.NewFlagSet("timecraft", flag.ExitOnError)
	flagSet.Usage = rootUsage

	v := flagSet.Bool("version", false, "")
	flagSet.BoolVar(v, "v", false, "")

	flagSet.Parse(args)

	if *v {
		fmt.Println("timecraft", version)
		os.Exit(0)
	}

	args = flagSet.Args()
	if len(args) == 0 {
		rootUsage()
		os.Exit(1)
	}
	switch args[0] {
	case "run":
		err = run(args[1:])
	default:
		err = fmt.Errorf("invalid command %q", args[0])
	}
	return
}

func runUsage() {
	fmt.Print(`timecraft run - Run a WebAssembly module

USAGE:
   timecraft run [OPTIONS]... <MODULE> [--] [ARGS]...

ARGS:
   <MODULE>
      The path of the WebAssembly module to run

   [ARGS]...
      Arguments to pass to the module

OPTIONS:
   --record <LOG>
      Record a trace of execution to a log at the specified path

   --dir <DIR>
      Grant access to the specified host directory

   --listen <ADDR>
      Grant access to a socket listening on the specified address

   --dial <ADDR>
      Grant access to a socket connected to the specified address

   --env <NAME=VAL>
      Pass an environment variable to the module

   --sockets <NAME>
      Enable a sockets extension, either {none, auto, path_open,
      wasmedgev1, wasmedgev2}

   --trace
      Enable logging of system calls (like strace)

   -h, --help
      Show this usage information
`)
}

func run(args []string) error {
	flagSet := flag.NewFlagSet("timecraft run", flag.ExitOnError)
	flagSet.Usage = runUsage

	var envs stringList
	var dirs stringList
	var listens stringList
	var dials stringList
	flagSet.Var(&envs, "env", "")
	flagSet.Var(&dirs, "dir", "")
	flagSet.Var(&listens, "listen", "")
	flagSet.Var(&dials, "dial", "")
	sockets := flagSet.String("sockets", "auto", "")
	trace := flagSet.Bool("trace", false, "")
	record := flagSet.String("record", "", "")

	flagSet.Parse(args)

	args = flagSet.Args()
	if len(args) == 0 {
		runUsage()
		os.Exit(1)
	}

	wasmFile := args[0]
	wasmName := filepath.Base(wasmFile)
	wasmCode, err := os.ReadFile(wasmFile)
	if err != nil {
		return fmt.Errorf("could not read WASM file '%s': %w", wasmFile, err)
	}

	args = args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	wasmModule, err := runtime.CompileModule(ctx, wasmCode)
	if err != nil {
		return err
	}

	builder := imports.NewBuilder().
		WithName(wasmName).
		WithArgs(args...).
		WithEnv(envs...).
		WithDirs(dirs...).
		WithListens(listens...).
		WithDials(dials...).
		WithSocketsExtension(*sockets, wasmModule).
		WithTracer(*trace, os.Stderr)

	if *record != "" {
		logFile, err := os.OpenFile(*record, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return fmt.Errorf("failed to open log file %q: %w", *record, err)
		}
		defer logFile.Close()

		logWriter := timemachine.NewLogWriter(logFile)

		var functions timemachine.FunctionIndex
		importedFunctions := wasmModule.ImportedFunctions()
		for _, f := range importedFunctions {
			moduleName, functionName, isImport := f.Import()
			if !isImport {
				continue
			}
			functions.Add(moduleName, functionName)
		}

		header := &timemachine.LogHeader{
			Runtime: timemachine.Runtime{
				Runtime:   "timecraft",
				Version:   version,
				Functions: functions.Functions(),
			},
			Process: timemachine.Process{
				ID:        timemachine.Hash{}, // TODO
				Image:     timemachine.SHA256(wasmCode),
				StartTime: time.Now(),
				Args:      args,
				Environ:   envs,
			},
			Compression: timemachine.Snappy, // TODO: make this configurable
		}

		if err := logWriter.WriteLogHeader(header); err != nil {
			return fmt.Errorf("failed to write log header: %w", err)
		}

		builder = builder.WithDecorators(timemachine.Capture[*wasi_snapshot_preview1.Module](functions, func(record timemachine.Record) {
			if _, err := logWriter.WriteRecordBatch([]timemachine.Record{record}); err != nil {
				panic(err)
			}
		}))
	}

	var system wasi.System
	ctx, system, err = builder.Instantiate(ctx, runtime)
	if err != nil {
		return err
	}
	defer system.Close(ctx)

	instance, err := runtime.InstantiateModule(ctx, wasmModule, wazero.NewModuleConfig())
	if err != nil {
		return err
	}
	return instance.Close(ctx)
}

type stringList []string

func (s stringList) String() string {
	return fmt.Sprintf("%v", []string(s))
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}
