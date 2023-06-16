package main

import (
	"context"
	"fmt"

	"github.com/stealthrocket/timecraft/internal/timecraft"
)

const versionUsage = `
Usage:	timecraft version

Options:
   -h, --help  Show this usage information
`

func version(ctx context.Context, args []string) error {
	flagSet := newFlagSet("timecraft version", versionUsage)
	if _, err := parseFlags(flagSet, args); err != nil {
		return err
	}
	fmt.Printf("timecraft %s\n", timecraft.Version())
	return nil
}
