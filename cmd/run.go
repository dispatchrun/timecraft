package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/sys"
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a WebAssembly module",
	Long:  `TODO`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:   cmdFunc(run),
}

func init() {
	rootCmd.AddCommand(runCmd)
}

func run(ctx context.Context, args []string) error {
	wasmFile := args[0]
	wasmName := filepath.Base(wasmFile)
	wasmCode, err := os.ReadFile(wasmFile)
	if err != nil {
		return fmt.Errorf("could not read WASM file '%s': %w", wasmFile, err)
	}

	environ := os.Environ()
	if os.Getenv("PWD") == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		environ = append(environ, "PWD="+wd)
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	builder := imports.NewBuilder().
		WithName(wasmName).
		WithArgs(args[1:]...).
		WithEnv(environ...).
		WithDirs("/").
		WithExit(func(ctx context.Context, exitCode int) error {
			panic(sys.NewExitError(uint32(exitCode)))
		})

	ctx, err = builder.Instantiate(ctx, runtime)
	if err != nil {
		return err
	}

	instance, err := runtime.Instantiate(ctx, wasmCode)
	if err != nil {
		switch e := err.(type) {
		case *sys.ExitError:
			if exitCode := e.ExitCode(); exitCode != 0 {
				os.Exit(int(exitCode))
			}
		default:
			return err
		}
	}
	return instance.Close(ctx)
}
