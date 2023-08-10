package main

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
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/timecraft"
	"golang.org/x/exp/maps"
)

const rootUsage = `timecraft - WebAssembly Time Machine

   timecraft is a WebAssembly runtime that provides advanced capabilities to the
   applications it runs, such as creating records of the program execution which
   are later replayable.

Example:

   $ timecraft run -- app.wasm
   f6e9acbc-0543-47df-9413-b99f569cfa3b
   ...

   $ timecraft replay f6e9acbc-0543-47df-9413-b99f569cfa3b
   ...

For a list of commands available, run 'timecraft help'.`

// Root is the timecraft entrypoint.
func Root(ctx context.Context, args ...string) int {
	if v := os.Getenv("TIMECRAFTCONFIG"); v != "" {
		timecraft.ConfigPath = human.Path(v)
	}

	var (
		// Secret options, we don't document them since they are only used for
		// development. Since they are not part of the public interface we may
		// remove or change the syntax at any time.
		cpuProfile human.Path
		memProfile human.Path
	)

	flagSet := newFlagSet("timecraft", helpUsage)
	customVar(flagSet, &cpuProfile, "cpuprofile")
	customVar(flagSet, &memProfile, "memprofile")

	if err := flagSet.Parse(args); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			fmt.Println(err)
		}
		return 0
	}

	if args = flagSet.Args(); len(args) == 0 {
		fmt.Println(rootUsage)
		return 0
	}

	if cpuProfile != "" {
		path, _ := cpuProfile.Resolve()
		f, err := os.Create(path)
		if err != nil {
			perrorf("WARN: could not create CPU profile: %s", err)
		} else {
			defer f.Close()
			_ = pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}
	}

	if memProfile != "" {
		path, _ := memProfile.Resolve()
		defer func() {
			f, err := os.Create(path)
			if err != nil {
				perrorf("WARN: could not create memory profile: %s", err)
			}
			defer f.Close()
			runtime.GC()
			_ = pprof.WriteHeapProfile(f)
		}()
	}

	var err error
	cmd, args := args[0], args[1:]
	switch cmd {
	case "config":
		err = config(ctx, args)
	case "describe":
		err = describe(ctx, args)
	case "export":
		err = export(ctx, args)
	case "get":
		err = get(ctx, args)
	case "help":
		err = help(ctx, args)
	case "logs":
		err = logs(ctx, args)
	case "profile":
		err = profile(ctx, args)
	case "run":
		err = run(ctx, args)
	case "replay":
		err = replay(ctx, args)
	case "trace":
		err = trace(ctx, args)
	case "version":
		err = version(ctx, args)
	default:
		err = unknown(ctx, cmd)
	}

	if errors.Is(err, flag.ErrHelp) {
		return 0
	}

	switch e := err.(type) {
	case nil:
		return 0
	case exitCode:
		return int(e)
	case timecraft.ExitError:
		return int(e)
	default:
		perrorf("ERR: timecraft %s: %s", cmd, err)
		return 1
	}
}

// exitCode is an error type returned from command functions to indicate the
// exit code that should be returned by the program.
type exitCode int

func (e exitCode) Error() string {
	return fmt.Sprintf("exit: %d", e)
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

func perror(args ...any) {
	fmt.Fprintln(os.Stderr, args...)
}

func perrorf(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
}

func newFlagSet(cmd, usage string) *flag.FlagSet {
	flagSet := flag.NewFlagSet(cmd, flag.ContinueOnError)
	flagSet.Usage = func() { fmt.Println(usage) }
	customVar(flagSet, &timecraft.ConfigPath, "c", "config")
	return flagSet
}

// parseFlags is a greedy parser which consumes all options known to f and
// returns the remaining arguments.
func parseFlags(f *flag.FlagSet, args []string) ([]string, error) {
	var vals []string
	for {
		if err := f.Parse(args); err != nil {
			if errors.Is(err, flag.ErrHelp) {
				return nil, err
			}
			return nil, exitCode(2)
		}
		if args = f.Args(); len(args) == 0 {
			return vals, nil
		}
		i := slices.IndexFunc(args, func(s string) bool {
			return strings.HasPrefix(s, "-")
		})
		if i < 0 {
			i = len(args)
		} else if args[i] == "-" {
			i++
		}
		if i == 0 {
			panic("parsing command line arguments did not error on " + args[0])
		}
		vals = append(vals, args[:i]...)
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

func parseProcessID(s string) (format.UUID, error) {
	processID, err := uuid.Parse(s)
	if err != nil {
		err = errors.New(`malformed process id passed as argument (not a UUID)`)
	}
	return processID, err
}
