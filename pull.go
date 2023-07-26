package main

import (
	"context"
	"errors"

	"github.com/stealthrocket/timecraft/internal/timecraft"
)

const pullUsage = `
Usage:	timecraft pull [options] [--] <image>

Options:
`

func pull(ctx context.Context, args []string) error {
	flagSet := newFlagSet("timecraft pull", pullUsage)
	if err := flagSet.Parse(args); err != nil {
		return err
	}
	args = flagSet.Args()
	if len(args) != 1 {
		return errors.New("missing image argument")
	}

	config, err := timecraft.LoadConfig()
	if err != nil {
		return err
	}

	images, err := timecraft.NewImages(config)
	if err != nil {
		return err
	}

	return images.Pull(args[0])
}
