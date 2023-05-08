package cmd

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wasi-go/systems/unix"
	"github.com/stealthrocket/wazergo"
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

	provider := &unix.System{
		Args:               append([]string{wasmName}, args[1:]...),
		Environ:            environ,
		Realtime:           realtime,
		RealtimePrecision:  time.Microsecond,
		Monotonic:          monotonic,
		MonotonicPrecision: time.Nanosecond,
		Rand:               rand.Reader,
		Exit: func(ctx context.Context, exitCode int) error {
			panic(sys.NewExitError(uint32(exitCode)))
		},
	}

	stdin, err := dup(0)
	if err != nil {
		return fmt.Errorf("opening stdin: %w", err)
	}

	stdout, err := dup(1)
	if err != nil {
		return fmt.Errorf("opening stdout: %w", err)
	}

	stderr, err := dup(2)
	if err != nil {
		return fmt.Errorf("opening stderr: %w", err)
	}

	root, err := syscall.Open("/", syscall.O_DIRECTORY, 0)
	if err != nil {
		return fmt.Errorf("opening root directory: %w", err)
	}

	provider.Preopen(stdin, "/dev/stdin", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.AllRights,
	})

	provider.Preopen(stdout, "/dev/stdout", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.AllRights,
	})

	provider.Preopen(stderr, "/dev/stderr", wasi.FDStat{
		FileType:   wasi.CharacterDeviceType,
		RightsBase: wasi.AllRights,
	})

	provider.Preopen(root, "/", wasi.FDStat{
		FileType:         wasi.DirectoryType,
		RightsBase:       wasi.AllRights,
		RightsInheriting: wasi.AllRights,
	})

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	ctx = wazergo.WithModuleInstance(ctx,
		wazergo.MustInstantiate(ctx, runtime,
			wasi_snapshot_preview1.HostModule,
			wasi_snapshot_preview1.WithWASI(provider),
		),
	)

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

var epoch = time.Now()

func monotonic(context.Context) (uint64, error) {
	return uint64(time.Since(epoch)), nil
}

func realtime(context.Context) (uint64, error) {
	return uint64(time.Now().UnixNano()), nil
}

func dup(fd int) (int, error) {
	newfd, err := syscall.Dup(fd)
	if err != nil {
		return -1, err
	}
	syscall.CloseOnExec(newfd)
	return newfd, nil
}
