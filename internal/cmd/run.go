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
	"github.com/stealthrocket/timecraft/internal/debug"

	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

const runUsage = `
Usage:	timecraft run [options] [--] <module> [args...]

Options:
   -d, --debug                    Start an interactive debugger
       --dial addr                Expose a socket connected to the specified address
   -e, --env name=value           Pass an environment variable to the guest module
   -h, --help                     Show this usage information
   -L, --listen addr              Expose a socket listening on the specified address
   -S, --sockets extension        Enable a sockets extension, one of none, auto, path_open, wasmedgev1, wasmedgev2 (default to auto)
   -R, --record                   Enable recording of the guest module execution
       --record-batch-size size   Number of records written per batch (default to 4096)
       --record-compression type  Compression to use when writing records, either snappy or zstd (default to zstd)
   -r, --registry path            Path to the timecraft registry (default to ~/.timecraft)
   -T, --trace                    Enable strace-like logging of host function calls
`

func run(ctx context.Context, args []string) error {
	var (
		envs         stringList
		listens      stringList
		dials        stringList
		batchSize    = 4096
		compression  = "zstd"
		sockets      = "auto"
		registryPath = "~/.timecraft"
		record       = false
		trace        = false
		debugger     = false
	)

	flagSet := newFlagSet("timecraft run", runUsage)
	customVar(flagSet, &envs, "e", "env")
	customVar(flagSet, &listens, "L", "listen")
	customVar(flagSet, &dials, "dial")
	stringVar(flagSet, &sockets, "S", "sockets")
	stringVar(flagSet, &registryPath, "r", "registry")
	boolVar(flagSet, &trace, "T", "trace")
	boolVar(flagSet, &record, "R", "record")
	boolVar(flagSet, &debugger, "d", "debug")
	intVar(flagSet, &batchSize, "record-batch-size")
	stringVar(flagSet, &compression, "record-compression")
	flagSet.Parse(args)

	envs = append(os.Environ(), envs...)
	args = flagSet.Args()

	if len(args) == 0 {
		return errors.New(`missing "--" separator before the module path`)
	}

	registry, err := createRegistry(registryPath)
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

	var debugREPL *debug.REPL
	if debugger {
		debugREPL = debug.NewREPL(os.Stdin, os.Stdout)
		ctx = debug.RegisterFunctionListener(ctx, debugREPL)
	}

	wasmModule, err := runtime.CompileModule(ctx, wasmCode)
	if err != nil {
		return err
	}
	defer wasmModule.Close(ctx)

	builder := imports.NewBuilder().
		WithName(wasmName).
		WithArgs(args...).
		WithEnv(envs...).
		WithDirs("/").
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

		module, err := registry.CreateModule(ctx, &format.Module{
			Code: wasmCode,
		})
		if err != nil {
			return err
		}

		runtime, err := registry.CreateRuntime(ctx, &format.Runtime{
			Version: currentVersion(),
		})
		if err != nil {
			return err
		}

		config, err := registry.CreateConfig(ctx, &format.Config{
			Runtime: runtime,
			Modules: []*format.Descriptor{module},
			Args:    append([]string{wasmName}, args...),
			Env:     envs,
		})
		if err != nil {
			return err
		}

		process, err := registry.CreateProcess(ctx, &format.Process{
			ID:        processID,
			StartTime: startTime,
			Config:    config,
		})
		if err != nil {
			return err
		}

		if err := registry.CreateLogManifest(ctx, processID, &format.Manifest{
			Process:   process,
			StartTime: startTime,
		}); err != nil {
			return err
		}

		logSegment, err := registry.CreateLogSegment(ctx, processID, 0)
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

	if debugger {
		system = debug.WASIListener(system, debugREPL)
	}

	return exec(ctx, runtime, wasmModule)
}

func exec(ctx context.Context, runtime wazero.Runtime, compiledModule wazero.CompiledModule) error {
	module, err := runtime.InstantiateModule(ctx, compiledModule, wazero.NewModuleConfig().
		WithStartFunctions())
	if err != nil {
		return err
	}
	defer module.Close(ctx)

	ctx, cancel := context.WithCancelCause(ctx)
	go func() {
		_, err := module.ExportedFunction("_start").Call(ctx)
		module.Close(ctx)
		cancel(err)
	}()

	<-ctx.Done()

	switch err := context.Cause(ctx).(type) {
	case nil:
	case *sys.ExitError:
		if exitCode := err.ExitCode(); exitCode != 0 {
			return ExitCode(exitCode)
		}
	default:
		return err
	}
	return nil
}
