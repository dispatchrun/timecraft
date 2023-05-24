package cmd

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/user"
	"path/filepath"
	"runtime/debug"
	"strings"

	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

// ExitCode is an error type returned from Root to indicate the exit code that
// should be returned by the program.
type ExitCode int

func (e ExitCode) Error() string {
	return fmt.Sprintf("exit: %d", e)
}

var version = "devel"

func init() {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		version = info.Main.Version
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

// Root is the timecraft entrypoint.
func Root(ctx context.Context, args []string) error {
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
		return run(ctx, args[1:])
	case "replay":
		return replay(ctx, args[1:])
	default:
		return fmt.Errorf("invalid command %q", args[0])
	}
}

type stringList []string

func (s stringList) String() string {
	return fmt.Sprintf("%v", []string(s))
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func createStore(path string) (*timemachine.Store, error) {
	path, err := resolvePath(path)
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(path, 0777); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	return openStore(path)
}

func openStore(path string) (*timemachine.Store, error) {
	path, err := resolvePath(path)
	if err != nil {
		return nil, err
	}
	dir, err := object.DirStore(path)
	if err != nil {
		return nil, err
	}
	return timemachine.NewStore(dir), nil
}

func resolvePath(path string) (string, error) {
	if strings.HasPrefix(path, "~") {
		u, err := user.Current()
		if err != nil {
			return "", err
		}
		path = filepath.Join(u.HomeDir, path[1:])
	}
	return path, nil
}
