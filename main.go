package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
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
   run     Run a WebAssembly module, and optionally trace execution
   replay  Replay a recorded trace of execution

OPTIONS:
   --pprof-addr <ADDR>
      Start a pprof server listening on the specified address

   -v, --version
      Print the version and exit

   -h, --help
      Show this usage information
`)
}

func root(args []string) error {
	flagSet := flag.NewFlagSet("timecraft", flag.ExitOnError)
	flagSet.Usage = rootUsage

	pprofAddr := flagSet.String("pprof-addr", "", "")
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

	if *pprofAddr != "" {
		go http.ListenAndServe(*pprofAddr, nil)
	}

	switch args[0] {
	case "run":
		return run(args[1:])
	case "replay":
		return replay(args[1:])
	default:
		return fmt.Errorf("invalid command %q", args[0])
	}
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

   --compression <TYPE>
      Compression to use when writing the log, either {snappy, zstd,
      none}. Default is snappy

   --batch-size <NUM>
      Number of records to accumulate in a batch before writing to the log

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
      wasmedgev1, wasmedgev2}. Default is auto

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
	logPath := flagSet.String("record", "", "")
	compression := flagSet.String("compression", "snappy", "")
	batchSize := flagSet.Int("batch-size", 1, "")

	flagSet.Parse(args)

	args = flagSet.Args()
	if len(args) == 0 {
		runUsage()
		os.Exit(1)
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

	if *logPath != "" {
		logFile, err := os.Create(*logPath)
		if err != nil {
			return fmt.Errorf("failed to open log file %q: %w", *logPath, err)
		}
		defer logFile.Close()

		logWriter := timemachine.NewLogWriter(logFile)

		bufferedLogWriter := timemachine.NewBufferedLogWriter(logWriter, *batchSize)
		defer bufferedLogWriter.Flush()

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
				Args:      append([]string{wasmName}, args...),
				Environ:   envs,
			},
		}

		switch strings.ToLower(*compression) {
		case "snappy":
			header.Compression = timemachine.Snappy
		case "zstd":
			header.Compression = timemachine.Zstd
		case "none", "":
			header.Compression = timemachine.Uncompressed
		default:
			return fmt.Errorf("invalid compression type %q", *compression)
		}

		if err := logWriter.WriteLogHeader(header); err != nil {
			return fmt.Errorf("failed to write log header: %w", err)
		}

		builder = builder.WithDecorators(timemachine.Capture[*wasi_snapshot_preview1.Module](functions, func(record timemachine.Record) {
			if err := bufferedLogWriter.WriteRecord(record); err != nil {
				panic(err) // TODO: better error handling
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

	wasmName, args := logHeader.Process.Args[0], logHeader.Process.Args[1:]
	envs := logHeader.Process.Environ

	builder := imports.NewBuilder().
		WithName(wasmName).
		WithArgs(args...).
		WithEnv(envs...)
	// TODO? WithDirs(dirs...).
	// TODO? WithListens(listens...).
	// TODO? WithDials(dials...).
	// TODO? WithSocketsExtension(*sockets, wasmModule).

	var functions timemachine.FunctionIndex
	importedFunctions := wasmModule.ImportedFunctions()
	for _, f := range importedFunctions {
		moduleName, functionName, isImport := f.Import()
		if !isImport {
			continue
		}
		functions.Add(moduleName, functionName)
	}
	if len(logHeader.Runtime.Functions) != len(functions.Functions()) {
		return fmt.Errorf("imported functions mismatch")
	}
	for i, fn := range functions.Functions() {
		if fn != logHeader.Runtime.Functions[i] {
			return fmt.Errorf("imported functions mismatch")
		}
	}

	records := timemachine.NewLogRecordIterator(logReader)

	builder = builder.WithDecorators(wazergo.DecoratorFunc[*wasi_snapshot_preview1.Module](func(moduleName string, f wazergo.Function[*wasi_snapshot_preview1.Module]) wazergo.Function[*wasi_snapshot_preview1.Module] {
		functionID, ok := functions.Lookup(moduleName, f.Name)
		if !ok {
			return f
		}
		return wazergo.Function[*wasi_snapshot_preview1.Module]{
			Name:    f.Name,
			Params:  f.Params,
			Results: f.Results,
			Func: func(module *wasi_snapshot_preview1.Module, ctx context.Context, mod api.Module, stack []uint64) {
				// TODO: better error handling
				if !records.Next() {
					if err := records.Error(); err != nil {
						panic(err)
					}
					panic("EOF")
				}
				record := records.Record()
				if recordFunctionID := record.Function(); recordFunctionID != functionID {
					panic(fmt.Sprintf("function ID mismatch: got %d, expect %d", functionID, recordFunctionID))
				}
				if len(f.Params) != record.NumParams() {
					panic(fmt.Sprintf("function param count mismatch: got %d, expect %d", len(f.Params), record.NumParams()))
				}
				for i := 0; i < record.NumParams(); i++ {
					if recordParam := record.ParamAt(i); recordParam != stack[i] {
						panic(fmt.Sprintf("function param %d mismatch: got %d, expect %d", i, stack[i], recordParam))
					}
				}
				if len(f.Results) != record.NumResults() {
					panic(fmt.Sprintf("function result count mismatch: got %d, expect %d", len(f.Results), record.NumResults()))
				}
				mutations := make([]timemachine.MemoryAccess, record.NumMemoryAccess())
				if err := record.ReadMemoryAccess(mutations); err != nil {
					panic(err)
				}
				memory := mod.Memory()
				for _, m := range mutations {
					b, ok := memory.Read(m.Offset, uint32(len(m.Memory)))
					if !ok {
						panic(fmt.Sprintf("unable to write %d bytes of memory to offset %d", len(m.Memory), m.Offset))
					}
					copy(b, m.Memory)
				}
				for i := 0; i < record.NumResults(); i++ {
					stack[i] = record.ResultAt(i)
				}

				// TODO: the record doesn't capture the fact that a host function
				//  didn't return. Hard-code this case for now.
				if moduleName == wasi_snapshot_preview1.HostModuleName && f.Name == "proc_exit" {
					panic(sys.NewExitError(uint32(stack[0])))
				}
			},
		}
	}))

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
