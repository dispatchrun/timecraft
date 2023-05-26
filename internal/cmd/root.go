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
	"io/fs"
	_ "net/http/pprof"
	"os"
	"os/user"
	"path/filepath"
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

const rootUsage = `timecraft - WebAssembly Time Machine

timecraft is a WebAssembly runtime which provides advanced capabilities to the
applications it runs, such as creating records of the program execution which
are later replayable.

Example:

   $ timecraft run --record -- app.wasm
   timecraft run: f6e9acbc-0543-47df-9413-b99f569cfa3b
   ...

   $ timecraft replay f6e9acbc-0543-47df-9413-b99f569cfa3b
   ...

For a list of commands available, run 'timecraft help'.`

type command struct {
	name     string
	usage    string
	function func(context.Context, []string) error
}

// Root is the timecraft entrypoint.
func Root(ctx context.Context, args []string) error {
	flagSet := newFlagSet("timecraft", helpUsage)
	flagSet.Parse(args)

	if args = flagSet.Args(); len(args) == 0 {
		fmt.Println(rootUsage)
		return ExitCode(1)
	}

	switch cmd, args := args[0], args[1:]; cmd {
	case "help":
		return help(ctx, args)
	case "run":
		return run(ctx, args)
	case "replay":
		return replay(ctx, args)
	case "version":
		return version(ctx, args)
	default:
		return unknown(ctx, cmd)
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

func newFlagSet(cmd, usage string) *flag.FlagSet {
	flagSet := flag.NewFlagSet(cmd, flag.ExitOnError)
	flagSet.Usage = func() { fmt.Println(usage) }
	return flagSet
}

func customVar(f *flag.FlagSet, dst flag.Value, short, long string) {
	if short != "" {
		f.Var(dst, short, "")
	}
	if long != "" {
		f.Var(dst, long, "")
	}
}

func stringVar(f *flag.FlagSet, dst *string, short, long string) {
	if short != "" {
		f.StringVar(dst, short, *dst, "")
	}
	if long != "" {
		f.StringVar(dst, long, *dst, "")
	}
}

func boolVar(f *flag.FlagSet, dst *bool, short, long string) {
	if short != "" {
		f.BoolVar(dst, short, *dst, "")
	}
	if long != "" {
		f.BoolVar(dst, long, *dst, "")
	}
}

func intVar(f *flag.FlagSet, dst *int, short, long string) {
	if short != "" {
		f.IntVar(dst, short, *dst, "")
	}
	if long != "" {
		f.IntVar(dst, long, *dst, "")
	}
}
