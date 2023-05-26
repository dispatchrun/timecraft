package cmd

// Notes on program structure
// --------------------------
//
// Timecraft uses subcommands to invoke specific functionalities of the program.
// Each subcommand is implemented by a function named after the command, in a
// file of the same name (e.g. the "help" command is implemented by the help
// function in help.go).
//
// The usage message for each command is declared by a constant starting with
// the command name and followed by the suffix "Usage". For example, the usage
// message for the "help" command is declared by the constant helpUsage.
//
// The usage message contains a "Usage:	timecraft <command>" section presenting
// the structure of the command. Note the tabulation separating "Usage:" and
// "timecraft".

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	_ "net/http/pprof"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"time"

	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

// ExitCode is an error type returned from Root to indicate the exit code that
// should be returned by the program.
type ExitCode int

func (e ExitCode) Error() string {
	return fmt.Sprintf("exit: %d", e)
}

const rootUsage = `timecraft - WebAssembly Time Machine

   timecraft is a WebAssembly runtime that provides advanced capabilities to the
   applications it runs, such as creating records of the program execution which
   are later replayable.

Example:

   $ timecraft run --record -- app.wasm
   timecraft run: f6e9acbc-0543-47df-9413-b99f569cfa3b
   ...

   $ timecraft replay f6e9acbc-0543-47df-9413-b99f569cfa3b
   ...

For a list of commands available, run 'timecraft help'.`

func init() {
	// TODO: do something better with logs
	log.SetOutput(io.Discard)
}

// Root is the timecraft entrypoint.
func Root(ctx context.Context, args []string) int {
	flagSet := newFlagSet("timecraft", helpUsage)
	flagSet.Parse(args)

	if args = flagSet.Args(); len(args) == 0 {
		fmt.Println(rootUsage)
		return 1
	}

	var err error
	cmd, args := args[0], args[1:]
	switch cmd {
	case "help":
		err = help(ctx, args)
	case "prof":
		err = prof(ctx, args)
	case "run":
		err = run(ctx, args)
	case "replay":
		err = replay(ctx, args)
	case "version":
		err = version(ctx, args)
	default:
		err = unknown(ctx, cmd)
	}

	switch e := err.(type) {
	case nil:
		return 0
	case ExitCode:
		return int(e)
	default:
		fmt.Printf("timecraft %s: %s\n", cmd, err)
		return 1
	}
}

type timestamp time.Time

func (ts timestamp) String() string {
	t := time.Time(ts)
	if t.IsZero() {
		return "start"
	}
	return t.Format(time.RFC3339)
}

func (ts *timestamp) Set(value string) error {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return err
	}
	*ts = timestamp(t)
	return nil
}

type stringList []string

func (s stringList) String() string {
	return fmt.Sprintf("%v", []string(s))
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func createRegistry(path string) (*timemachine.Registry, error) {
	path, err := resolvePath(path)
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(path, 0777); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	return openRegistry(path)
}

func openRegistry(path string) (*timemachine.Registry, error) {
	path, err := resolvePath(path)
	if err != nil {
		return nil, err
	}
	dir, err := object.DirStore(path)
	if err != nil {
		return nil, err
	}
	return timemachine.NewRegistry(dir), nil
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

func newFlagSet(cmd, usage string) *flag.FlagSet {
	flagSet := flag.NewFlagSet(cmd, flag.ExitOnError)
	flagSet.Usage = func() { fmt.Println(usage) }
	return flagSet
}

func customVar(f *flag.FlagSet, dst flag.Value, name string, alias ...string) {
	f.Var(dst, name, "")
	for _, name := range alias {
		f.Var(dst, name, "")
	}
}

func durationVar(f *flag.FlagSet, dst *time.Duration, name string, alias ...string) {
	setFlagVar(f.DurationVar, dst, name, alias)
}

func stringVar(f *flag.FlagSet, dst *string, name string, alias ...string) {
	setFlagVar(f.StringVar, dst, name, alias)
}

func boolVar(f *flag.FlagSet, dst *bool, name string, alias ...string) {
	setFlagVar(f.BoolVar, dst, name, alias)
}

func intVar(f *flag.FlagSet, dst *int, name string, alias ...string) {
	setFlagVar(f.IntVar, dst, name, alias)
}

func float64Var(f *flag.FlagSet, dst *float64, name string, alias ...string) {
	setFlagVar(f.Float64Var, dst, name, alias)
}

func setFlagVar[T any](set func(*T, string, T, string), dst *T, name string, alias []string) {
	set(dst, name, *dst, "")
	for _, name := range alias {
		set(dst, name, *dst, "")
	}
}
