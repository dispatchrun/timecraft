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
	"strings"

	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
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
	_ = flagSet.Parse(args)

	if args = flagSet.Args(); len(args) == 0 {
		fmt.Println(rootUsage)
		return 1
	}

	var err error
	cmd, args := args[0], args[1:]
	switch cmd {
	case "describe":
		err = describe(ctx, args)
	case "export":
		err = export(ctx, args)
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

func setEnum[T ~string](enum *T, typ string, value string, options ...string) error {
	for _, option := range options {
		if option == value {
			*enum = T(option)
			return nil
		}
	}
	return fmt.Errorf("unsupported %s: %q (not one of %s)", typ, value, strings.Join(options, ", "))
}

type compression string

func (c compression) String() string {
	return string(c)
}

func (c *compression) Set(value string) error {
	return setEnum(c, "compression type", value, "snappy", "zstd", "none")
}

type sockets string

func (s sockets) String() string {
	return string(s)
}

func (s *sockets) Set(value string) error {
	return setEnum(s, "sockets extension", value, "none", "auto", "path_open", "wasmedgev1", "wasmedgev2")
}

type outputFormat string

func (o outputFormat) String() string {
	return string(o)
}

func (o *outputFormat) Set(value string) error {
	return setEnum(o, "output format", value, "text", "json", "yaml")
}

type stringList []string

func (s stringList) String() string {
	return fmt.Sprintf("%v", []string(s))
}

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

type stringMap map[string]string

func (m stringMap) String() string {
	b := new(strings.Builder)
	keys := maps.Keys(m)
	slices.Sort(keys)
	for i, k := range keys {
		if i != 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte(':')
		b.WriteString(m[k])
	}
	return b.String()
}

func (m stringMap) Set(value string) error {
	k, v, _ := strings.Cut(value, ":")
	k = strings.TrimSpace(k)
	v = strings.TrimSpace(v)
	m[k] = v
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

// parseFlags is a greedy parser which consumes all options known to f and
// returns the remaining arguments.
func parseFlags(f *flag.FlagSet, args []string) []string {
	var unknownArgs []string
	for {
		// The flag set is constructed with ExitOnError, it should never error.
		if err := f.Parse(args); err != nil {
			panic(err)
		}
		if args = f.Args(); len(args) == 0 {
			return unknownArgs
		}
		i := slices.IndexFunc(args, func(s string) bool {
			return strings.HasPrefix(s, "-")
		})
		if i < 0 {
			i = len(args)
		}
		if i == 0 {
			panic("parsing command line arguments did not error on " + args[0])
		}
		unknownArgs = append(unknownArgs, args[:i]...)
		args = args[i:]
	}
}

func boolVar(f *flag.FlagSet, dst *bool, name string, alias ...string) {
	f.BoolVar(dst, name, *dst, "")
	for _, name := range alias {
		f.BoolVar(dst, name, *dst, "")
	}
}

func customVar(f *flag.FlagSet, dst flag.Value, name string, alias ...string) {
	f.Var(dst, name, "")
	for _, name := range alias {
		f.Var(dst, name, "")
	}
}
