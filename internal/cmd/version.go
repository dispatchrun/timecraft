package cmd

import (
	"context"
	"fmt"
	"runtime/debug"
)

const versionUsage = `
Usage:	timecraft version

Options:
   -h, --help  Show this usage information
`

func version(ctx context.Context, args []string) error {
	flagSet := newFlagSet("timecraft version", versionUsage)
	flagSet.Parse(args)
	fmt.Printf("timecraft %s\n", currentVersion())
	return nil
}

func currentVersion() string {
	version := "devel"
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		version = info.Main.Version
	}
	return version
}
