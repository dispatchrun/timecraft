package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"

	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
)

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

	version := flagSet.Bool("version", false, "")
	flagSet.BoolVar(version, "v", false, "")

	flagSet.Parse(args)

	if *version {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
			fmt.Println("timecraft", info.Main.Version)
		} else {
			fmt.Println("timecraft", "devel")
		}
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

	ctx, err = builder.Instantiate(ctx, runtime)
	if err != nil {
		return err
	}

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
