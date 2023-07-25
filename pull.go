package main

import (
	"context"
	"errors"
	"io/fs"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stealthrocket/timecraft/internal/timecraft"
)

const pullUsage = `
Usage:	timecraft pull [options] [--] <image>

Options:
`

func pull(ctx context.Context, args []string) error {
	flagSet := newFlagSet("timecraft run", runUsage)
	if err := flagSet.Parse(args); err != nil {
		return err
	}
	args = flagSet.Args()
	if len(args) != 1 {
		return errors.New("missing image argument")
	}

	path, err := timecraft.ImagesPath.Resolve()
	if err != nil {
		return err
	}

	if err := createDirectory(path); err != nil {
		return err
	}

	ref, err := name.ParseReference(args[0])
	if err != nil {
		return err
	}

	opts := []layout.Option{}
	opts = append(opts, layout.WithAnnotations(map[string]string{
		"org.opencontainers.image.ref.name": ref.Name(),
		"name":                              args[0],
	}))

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}

	p, err := layout.FromPath(path)
	if err != nil {
		p, err = layout.Write(path, empty.Index)
		if err != nil {
			return err
		}
	}

	if err := p.ReplaceImage(
		img,
		match.Annotation("org.opencontainers.image.ref.name", ref.Name()),
		opts...,
	); err != nil {
		return err
	}
	return nil
}

func createDirectory(path string) error {
	if err := os.MkdirAll(path, 0777); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}
