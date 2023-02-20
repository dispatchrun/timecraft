package cmd

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/segmentio/parquet-go"
	"github.com/spf13/cobra"
	"github.com/stealthrocket/plugins/modules/wasi_experimental_http"
	"github.com/stealthrocket/plugins/modules/wasi_snapshot_preview1"
	"github.com/stealthrocket/plugins/wasm"
	"github.com/stealthrocket/timecraft/pkg/timecraft"
	"github.com/tetratelabs/wazero"
)

func init() {
	rootCmd.AddCommand(replayCmd)
}

var replayCmd = &cobra.Command{
	Use:   "replay",
	Short: "Replays the execution of a WebAssembly module from a record file",
	Long:  ``,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	Run:   cmdFunc(replay),
}

func replay(ctx context.Context, args []string) error {
	f, err := os.Open(args[0])
	if err != nil {
		return err
	}
	defer f.Close()

	buffer := new(bytes.Buffer)
	module := new(bytes.Buffer)
	config := new(moduleConfig)
	records := []timecraft.Record{}

	archive := tar.NewReader(bufio.NewReader(f))
	for {
		h, err := archive.Next()
		if err != nil {
			if err != io.EOF {
				return err
			}
			break
		}
		switch h.Name {
		case "module.wasm":
			_, err = io.Copy(module, archive)
		case "config.json":
			err = json.NewDecoder(archive).Decode(config)
		case "records.parquet":
			buffer.Reset()
			_, err = buffer.ReadFrom(archive)
			if err == nil {
				records, err = parquet.Read[timecraft.Record](bytes.NewReader(buffer.Bytes()), int64(buffer.Len()))
			}
		}
		if err != nil {
			return fmt.Errorf("%s: %w", h.Name, err)
		}
	}

	index := 0
	playback := func() *timecraft.Record {
		if index < len(records) {
			r := &records[index]
			index++
			return r
		}
		return nil
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compilation := wasm.NewCompilationContext(ctx, runtime)
	instantiation := wasm.NewInstantiationContext(ctx, runtime)

	preview1, err := wasm.Compile(compilation, wasi_snapshot_preview1.NewPlugin(),
		timecraft.Replay[*wasi_snapshot_preview1.Module](playback),
	)
	if err != nil {
		return err
	}
	experimentalHTTP, err := wasm.Compile(compilation, wasi_experimental_http.NewPlugin(),
		timecraft.Replay[*wasi_experimental_http.Module](playback),
	)
	if err != nil {
		return err
	}
	compiledModule, err := runtime.CompileModule(ctx, module.Bytes())
	if err != nil {
		return err
	}

	wasiModule, err := wasm.Instantiate(instantiation, preview1,
		wasi_snapshot_preview1.SetArgs(config.Cmd),
		wasi_snapshot_preview1.SetEnv(config.Env),
		wasi_snapshot_preview1.SetStdout(os.Stdout),
		wasi_snapshot_preview1.SetStderr(os.Stderr),
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
			WithName(config.Cmd[0]).
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
