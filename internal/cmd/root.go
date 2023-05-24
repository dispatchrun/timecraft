package cmd

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
)

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
func Root(args []string) error {
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
		return run(args[1:])
	case "replay":
		return replay(args[1:])
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
