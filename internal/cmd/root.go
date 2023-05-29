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
	"time"

	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/print/human"
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
func Root(ctx context.Context, args ...string) int {
	flagSet := newFlagSet("timecraft", helpUsage)
	parseFlags(flagSet, args)

	if args = flagSet.Args(); len(args) == 0 {
		fmt.Println(rootUsage)
		return 1
	}

	var err error
	cmd, args := args[0], args[1:]
	switch cmd {
	case "get":
		err = get(ctx, args)
	case "help":
		err = help(ctx, args)
	case "profile":
		err = profile(ctx, args)
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
		fmt.Printf("ERR: timecraft %s: %s\n", cmd, err)
		return 1
	}
}

type outputFormat string

func (o outputFormat) String() string {
	return string(o)
}

func (o *outputFormat) Set(value string) error {
	switch value {
	case "text", "json", "yaml":
		*o = outputFormat(value)
		return nil
	default:
		return fmt.Errorf("unsupported output format: %q", value)
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

func createRegistry(path human.Path) (*timemachine.Registry, error) {
	p, err := path.Resolve()
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(p, 0777); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	return openRegistry(human.Path(p))
}

func openRegistry(path human.Path) (*timemachine.Registry, error) {
	p, err := path.Resolve()
	if err != nil {
		return nil, err
	}
	store, err := object.DirStore(p)
	if err != nil {
		return nil, err
	}
	registry := &timemachine.Registry{
		Store: store,
	}
	return registry, nil
}

func newFlagSet(cmd, usage string) *flag.FlagSet {
	flagSet := flag.NewFlagSet(cmd, flag.ExitOnError)
	flagSet.Usage = func() { fmt.Println(usage) }
	return flagSet
}

func parseFlags(f *flag.FlagSet, args []string) {
	// The flag set is consutrcted with ExitOnError, it should never error.
	if err := f.Parse(args); err != nil {
		panic(err)
	}
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
