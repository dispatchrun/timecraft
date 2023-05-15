package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/debug"
)

func usage() {
	fmt.Print(`timecraft - A time machine for production

USAGE:
   timecraft [OPTIONS]... <COMMAND> [ARGS]...

COMMANDS:
   record  Record a trace of execution
   replay  Replay a trace of execution

OPTIONS:
   -v, --version
      Print the version and exit

   -h, --help
      Show this usage information
`)
}

func main() {
	root := flag.NewFlagSet("timecraft", flag.ExitOnError)
	root.Usage = usage

	version := root.Bool("version", false, "")
	root.BoolVar(version, "v", false, "")

	root.Parse(os.Args[1:])

	if *version {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
			fmt.Println("timecraft", info.Main.Version)
		} else {
			fmt.Println("timecraft", "devel")
		}
		os.Exit(0)
	}

	args := root.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}
	switch args[0] {
	case "record":
		// TODO
	case "replay":
		// TODO
	}
}
