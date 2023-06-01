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
	"flag"
	"fmt"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
	"strings"

	"github.com/stealthrocket/timecraft/internal/print/human"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
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

// root is the timecraft entrypoint.
func root(args ...string) int {
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
	_ = flagSet.Parse(args)

	if args = flagSet.Args(); len(args) == 0 {
		fmt.Println(rootUsage)
		return 0
	}

	if cpuProfile != "" {
		path, _ := cpuProfile.Resolve()
		f, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: could not create CPU profile: %s\n", err)
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
				fmt.Fprintf(os.Stderr, "WARN: could not create memory profile: %s\n", err)
			}
			defer f.Close()
			runtime.GC()
			_ = pprof.WriteHeapProfile(f)
		}()
	}

	cmd, args := args[0], args[1:]

run_command:
	ctx := context.Background()

	var err error
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
	case exitCode:
		return int(e)
	case restart:
		goto run_command
	case usage:
		fmt.Fprintf(os.Stderr, "%s\n", e)
		return 2
	default:
		fmt.Fprintf(os.Stderr, "ERR: timecraft %s: %s\n", cmd, err)
		return 1
	}
}

// exitCode is an error type returned from command functions to indicate the
// exit code that should be returned by the program.
type exitCode int

func (e exitCode) Error() string {
	return fmt.Sprintf("exit: %d", e)
}

// restart is an error type returned from command functions to indicate
// that a command should be restarted.
type restart struct{}

func (restart) Error() string { return "restart" }

// usage is an error type returned from command functions to indicate a usage
// error.
//
// Usage erors cause the program to exist with status code 2.
type usage string

func usageError(msg string, args ...any) error {
	return usage(fmt.Sprintf(msg, args...))
}

func (e usage) Error() string {
	return string(e)
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

func newFlagSet(cmd, usage string) *flag.FlagSet {
	usage = strings.TrimSpace(usage)
	flagSet := flag.NewFlagSet(cmd, flag.ExitOnError)
	flagSet.Usage = func() { fmt.Println(usage) }
	customVar(flagSet, &configPath, "c", "config")
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
		} else if args[i] == "-" {
			i++
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
