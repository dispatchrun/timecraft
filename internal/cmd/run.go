package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
)

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
   --compression <TYPE>
      Compression to use when writing the log, either {snappy, zstd,
      none}. Default is zstd

   --batch-size <NUM>
      Number of records to accumulate in a batch before writing to
      the log. Default is 1024

   --dir <DIR>
      Grant access to the specified host directory

   --listen <ADDR>
      Grant access to a socket listening on the specified address

   --dial <ADDR>
      Grant access to a socket connected to the specified address

   --env <NAME=VAL>
      Pass an environment variable to the module

   --record
      Enable recording of the module execution

   --sockets <NAME>
      Enable a sockets extension, either {none, auto, path_open,
      wasmedgev1, wasmedgev2}. Default is auto

   --store <PATH>
      Path to the directory where the timecraft object store is available
      (default to ~/.timecraft)

   --trace
      Enable logging of system calls (like strace)

   -h, --help
      Show this usage information
`)
}

func run(ctx context.Context, args []string) error {
	flagSet := flag.NewFlagSet("timecraft run", flag.ExitOnError)
	flagSet.Usage = runUsage

	var (
		envs        stringList
		dirs        stringList
		listens     stringList
		dials       stringList
		batchSize   int
		compression string
		sockets     string
		store       string
		record      bool
		trace       bool
	)
	flagSet.Var(&envs, "env", "")
	flagSet.Var(&dirs, "dir", "")
	flagSet.Var(&listens, "listen", "")
	flagSet.Var(&dials, "dial", "")
	flagSet.StringVar(&compression, "compression", "zstd", "")
	flagSet.StringVar(&sockets, "sockets", "auto", "")
	flagSet.StringVar(&store, "store", "~/.timecraft", "")
	flagSet.BoolVar(&trace, "trace", false, "")
	flagSet.BoolVar(&record, "record", false, "")
	flagSet.IntVar(&batchSize, "batch-size", 4096, "")
	flagSet.Parse(args)

	args = flagSet.Args()
	if len(args) == 0 {
		runUsage()
		return ExitCode(1)
	}

	timestore, err := createStore(store)
	if err != nil {
		return err
	}

	wasmPath := args[0]
	wasmName := filepath.Base(wasmPath)
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("could not read WASM file '%s': %w", wasmPath, err)
	}

	args = args[1:]
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	wasmModule, err := runtime.CompileModule(ctx, wasmCode)
	if err != nil {
		return err
	}
	defer wasmModule.Close(ctx)

	builder := imports.NewBuilder().
		WithName(wasmName).
		WithArgs(args...).
		WithEnv(envs...).
		WithDirs(dirs...).
		WithListens(listens...).
		WithDials(dials...).
		WithSocketsExtension(sockets, wasmModule).
		WithTracer(trace, os.Stderr)

	if record {
		var c timemachine.Compression
		switch strings.ToLower(compression) {
		case "snappy":
			c = timemachine.Snappy
		case "zstd":
			c = timemachine.Zstd
		case "none", "":
			c = timemachine.Uncompressed
		default:
			return fmt.Errorf("invalid compression type %q", compression)
		}

		processID := uuid.New()
		startTime := time.Now()

		module, err := timestore.CreateModule(ctx, format.Module(wasmCode))
		if err != nil {
			return err
		}

		runtime, err := timestore.CreateRuntime(ctx, &format.Runtime{
			Version: version,
		})
		if err != nil {
			return err
		}

		config, err := timestore.CreateConfig(ctx, &format.Config{
			Runtime: runtime,
			Modules: []*format.Descriptor{module},
			Args:    append([]string{wasmName}, args...),
			Env:     envs,
		})
		if err != nil {
			return err
		}

		process, err := timestore.CreateProcess(ctx, &format.Process{
			ID:        processID,
			StartTime: startTime,
			Config:    config,
		})
		if err != nil {
			return err
		}

		if err := timestore.CreateLogManifest(ctx, processID, &format.Manifest{
			Process:   process,
			StartTime: startTime,
		}); err != nil {
			return err
		}

		logSegment, err := timestore.CreateLogSegment(ctx, processID, 0)
		if err != nil {
			return err
		}
		defer logSegment.Close()
		logWriter := timemachine.NewLogWriter(logSegment)

		recordWriter := timemachine.NewLogRecordWriter(logWriter, batchSize, c)
		defer recordWriter.Flush()

		builder = builder.WithWrappers(func(s wasi.System) wasi.System {
			return wasicall.NewRecorder(s, startTime, func(record *timemachine.RecordBuilder) {
				if err := recordWriter.WriteRecord(record); err != nil {
					panic(err)
				}
			})
		})

		defer fmt.Println(processID)
	}

	var system wasi.System
	ctx, system, err = builder.Instantiate(ctx, runtime)
	if err != nil {
		return err
	}
	defer system.Close(ctx)

	instance, err := runtime.InstantiateModule(ctx, wasmModule, wazero.NewModuleConfig().
		WithStartFunctions())
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		_, err := instance.ExportedFunction("_start").Call(ctx)
		cancel(err)
	}()
	if <-ctx.Done(); ctx.Err() != nil {
		fmt.Println()
	}
	return instance.Close(ctx)
}
