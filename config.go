package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/stealthrocket/timecraft/internal/timecraft"
	"gopkg.in/yaml.v3"
)

const configUsage = `
Usage:	timecraft config [options]

Options:
   -c, --config path    Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
       --edit           Open $EDITOR to edit the configuration
   -h, --help           Show usage information
   -o, --output format  Output format, one of: text, json, yaml
`

func config(ctx context.Context, args []string) error {
	var (
		edit   bool
		output = outputFormat("text")
	)

	flagSet := newFlagSet("timecraft config", configUsage)
	boolVar(flagSet, &edit, "edit")
	customVar(flagSet, &output, "o", "output")

	if _, err := parseFlags(flagSet, args); err != nil {
		return err
	}

	r, path, err := timecraft.OpenConfig()
	if err != nil {
		return err
	}
	defer r.Close()

	if edit {
		editor := os.Getenv("EDITOR")
		if editor == "" {
			return errors.New(`$EDITOR is not set`)
		}
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}

		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			if !errors.Is(err, fs.ErrExist) {
				return err
			}
		}

		tmp, err := createTempFile(path, r)
		if err != nil {
			return err
		}
		defer os.Remove(tmp)

		p, err := os.StartProcess(shell, []string{shell, "-c", editor + " " + tmp}, &os.ProcAttr{
			Files: []*os.File{
				0: os.Stdin,
				1: os.Stdout,
				2: os.Stderr,
			},
		})
		if err != nil {
			return err
		}
		if _, err := p.Wait(); err != nil {
			return err
		}
		f, err := os.Open(tmp)
		if err != nil {
			return err
		}
		defer f.Close()
		if _, err := timecraft.ReadConfig(f); err != nil {
			return fmt.Errorf("not applying configuration updates because the file has a syntax error: %w", err)
		}
		if err := os.Rename(tmp, path); err != nil {
			return err
		}
	}

	config, err := timecraft.LoadConfig()
	if err != nil {
		return err
	}

	w := io.Writer(os.Stdout)
	for {
		switch output {
		case "json":
			e := json.NewEncoder(w)
			e.SetEscapeHTML(false)
			e.SetIndent("", "  ")
			_ = e.Encode(config)
		case "yaml":
			e := yaml.NewEncoder(w)
			e.SetIndent(2)
			_ = e.Encode(config)
			_ = e.Close()
		default:
			r, _, err := timecraft.OpenConfig()
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					output = "yaml"
					continue
				}
				return err
			}
			defer r.Close()
			_, _ = io.Copy(w, r)
		}
		return nil
	}
}

func createTempFile(path string, r io.Reader) (string, error) {
	dir, file := filepath.Split(path)
	w, err := os.CreateTemp(dir, "."+file+".*")
	if err != nil {
		return "", err
	}
	defer w.Close()
	_, err = io.Copy(w, r)
	return w.Name(), err
}
