package cmd

import (
	"context"
	"errors"
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

const runUsage = `
Usage:	timecraft run [options] [--] <module> [args...]

Options:
   -B, --batch-size size    Number of records written per batch (default to 4096)
   -C, --compression type   Compression to use when writing records, either snappy or zstd (default to zstd)
   -D, --dial addr          Expose a socket connected to the specified address
   -d, --dir path           Expose the directory to the guest module
   -e, --env name=value     Pass an environment variable to the guest module
   -h, --help               Show this usage information
   -L, --listen addr        Expose a socket listening on the specified address
   -S, --sockets extension  Enable a sockets extension, one of none, auto, path_open, wasmedgev1, wasmedgev2 (default to auto)
       --store path         Path to the timecraft object store (default to ~/.timecraft)
   -R, --record             Enable recording of the guest module execution
   -T, --trace              Enable strace-like logging of host function calls
`

func run(ctx context.Context, args []string) error {
	var (
		envs        stringList
		dirs        stringList
		listens     stringList
		dials       stringList
		batchSize   = 4096
		compression = "zstd"
		sockets     = "auto"
		store       = "~/.timecraft"
		record      = false
		trace       = false
	)

	flagSet := newFlagSet("timecraft run", runUsage)
	customVar(flagSet, &envs, "e", "env")
	customVar(flagSet, &dirs, "d", "dir")
	customVar(flagSet, &listens, "L", "listen")
	customVar(flagSet, &dials, "D", "dial")
	stringVar(flagSet, &compression, "C", "compression")
	stringVar(flagSet, &sockets, "S", "sockets")
	stringVar(flagSet, &store, "", "store")
	boolVar(flagSet, &trace, "T", "trace")
	boolVar(flagSet, &record, "R", "record")
	intVar(flagSet, &batchSize, "B", "batch-size")
	flagSet.Parse(args)

	if args = flagSet.Args(); len(args) == 0 {
		return errors.New(`missing "--" separator before the module path`)
	}

	timestore, err := createStore(store)
	if err != nil {
		return err
	}

	wasmPath := args[0]
	wasmName := filepath.Base(wasmPath)
	wasmCode, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("could not read wasm file '%s': %w", wasmPath, err)
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

		module, err := timestore.CreateModule(ctx, &format.Module{
			Code: wasmCode,
		})
		if err != nil {
			return err
		}

		runtime, err := timestore.CreateRuntime(ctx, &format.Runtime{
			Version: currentVersion(),
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

		fmt.Println("timecraft run:", processID)
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
