package cmd

import (
	"archive/tar"
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/segmentio/parquet-go"
	"github.com/spf13/cobra"
	"github.com/stealthrocket/plugins/modules/wasi_experimental_http"
	"github.com/stealthrocket/plugins/modules/wasi_snapshot_preview1"
	"github.com/stealthrocket/plugins/wasm"
	"github.com/stealthrocket/timecraft/pkg/timecraft"
	"github.com/tetratelabs/wazero"
)

var (
	record bool
	output string
)

func init() {
	rootCmd.AddCommand(runCmd)

	flags := runCmd.Flags()
	flags.BoolVarP(&record, "record", "R", false, "Record the program execution")
	flags.StringVarP(&output, "output", "O", "", "Location where to save the program records")
}

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run a WebAssembly module",
	Long:  ``,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	Run:   cmdFunc(run),
}

func run(ctx context.Context, args []string) error {
	var environ = os.Environ()
	var capture func(context.Context, *timecraft.Record)

	module, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	if record {
		if output == "" {
			output = filepath.Base(args[0]) + ".tar"
		}
		records := []timecraft.Record{}
		defer func() { writeRecordArchive(module, args, environ, records) }()

		capture = func(ctx context.Context, rec *timecraft.Record) {
			records = append(records, *rec)
		}
	}

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	epoch := time.Now()
	compilation := wasm.NewCompilationContext(ctx, runtime)
	instantiation := wasm.NewInstantiationContext(ctx, runtime)

	preview1, err := wasm.Compile(compilation, wasi_snapshot_preview1.NewPlugin(),
		timecraft.Capture[*wasi_snapshot_preview1.Module](epoch, capture),
	)
	if err != nil {
		return err
	}
	experimentalHTTP, err := wasm.Compile(compilation, wasi_experimental_http.NewPlugin(),
		timecraft.Capture[*wasi_experimental_http.Module](epoch, capture),
	)
	if err != nil {
		return err
	}
	compiledModule, err := runtime.CompileModule(ctx, module)
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

type moduleConfig struct {
	Cmd []string `json:"cmd"`
	Env []string `json:"env"`
}

func writeRecordArchive(module []byte, cmd, env []string, records []timecraft.Record) error {
	buf1 := new(bytes.Buffer)
	buf2 := new(bytes.Buffer)

	json.NewEncoder(buf1).Encode(moduleConfig{
		Cmd: cmd,
		Env: env,
	})
	parquet.Write(buf2, records)
	now := time.Now()

	f, err := os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	b := bufio.NewWriter(f)
	defer b.Flush()

	w := tar.NewWriter(b)
	defer w.Close()

	if err := writeRecordFile(w, "module.wasm", module, 0755, now); err != nil {
		return err
	}
	if err := writeRecordFile(w, "config.json", buf1.Bytes(), 0644, now); err != nil {
		return err
	}
	if err := writeRecordFile(w, "records.parquet", buf2.Bytes(), 0644, now); err != nil {
		return err
	}
	return nil
}

func writeRecordFile(w *tar.Writer, name string, data []byte, mode int64, now time.Time) error {
	if err := w.WriteHeader(&tar.Header{
		Typeflag:   tar.TypeReg,
		Name:       name,
		Size:       int64(len(data)),
		Mode:       mode,
		ModTime:    now,
		AccessTime: now,
		ChangeTime: now,
		Format:     tar.FormatPAX,
	}); err != nil {
		return err
	}
	_, err := w.Write(data)
	return err
}
