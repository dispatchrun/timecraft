package cmd

import (
	"context"
	"crypto/rand"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"github.com/stealthrocket/plugins/modules/wasi_experimental_http"
	"github.com/stealthrocket/plugins/modules/wasi_snapshot_preview1"
	"github.com/stealthrocket/plugins/wasm"
	"github.com/tetratelabs/wazero"
)

var (
	environ []string
)

func init() {
	rootCmd.AddCommand(runCmd)

	flags := runCmd.Flags()
	flags.StringArrayVar(&environ, "env", nil, "list of environment variables to expose t othe test")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a WebAssembly module",
	Long:  ``,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:   cmdFunc(run),
}

func run(ctx context.Context, args []string) error {
	binary, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compilation := wasm.NewCompilationContext(ctx, runtime)
	instantiation := wasm.NewInstantiationContext(ctx, runtime)

	preview1, err := wasm.Compile(compilation, wasi_snapshot_preview1.NewPlugin())
	if err != nil {
		return err
	}
	experimentalHTTP, err := wasm.Compile(compilation, wasi_experimental_http.NewPlugin())
	if err != nil {
		return err
	}
	compiledModule, err := runtime.CompileModule(ctx, binary)
	if err != nil {
		return err
	}

	wasiModule, err := wasm.Instantiate(instantiation, preview1,
		wasi_snapshot_preview1.SetArgs(args),
		wasi_snapshot_preview1.SetEnv(environ),
		wasi_snapshot_preview1.SetStdin(os.Stdin),
		wasi_snapshot_preview1.SetStdout(os.Stdout),
		wasi_snapshot_preview1.SetStderr(os.Stderr),
		wasi_snapshot_preview1.SetRand(rand.Reader),
		wasi_snapshot_preview1.SetClock(wasi_snapshot_preview1.NewRealtimeClock()),
		wasi_snapshot_preview1.SetClock(wasi_snapshot_preview1.NewMonotonicClock()),
	)
	if err != nil {
		return err
	}
	defer wasiModule.Close(ctx)

	httpModule, err := wasm.Instantiate(instantiation, experimentalHTTP)
	if err != nil {
		return err
	}
	defer httpModule.Close(ctx)

	moduleInstance, err := runtime.InstantiateModule(ctx, compiledModule,
		wazero.NewModuleConfig().
			WithName(args[0]).
			WithStartFunctions(),
	)
	if err != nil {
		return err
	}
	defer moduleInstance.Close(ctx)
	callContext := wasm.NewCallContext(ctx, instantiation)
	_, err = moduleInstance.ExportedFunction("_start").Call(callContext)
	return err
}

func internalServerError(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}
