package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"gopkg.in/yaml.v3"
)

const configUsage = `
Usage:	timecraft config [options]

Options:
   -c, --config         Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
       --edit           Open $EDITOR to edit the configuration
   -h, --help           Show usage information
   -o, --output format  Output format, one of: text, json, yaml
`

var (
	// Path to the timecraft configuration; this is set to the value of the
	// TIMECRAFTCONFIG environment variable if it exists, and can be overwritten
	// by the --config option.
	configPath human.Path = "~/.timecraft/config.yaml"
)

func init() {
	if v := os.Getenv("TIMECRAFTCONFIG"); v != "" {
		configPath = human.Path(v)
	}
}

func config(ctx context.Context, args []string) error {
	var (
		edit   bool
		output = outputFormat("text")
	)

	flagSet := newFlagSet("timecraft config", configUsage)
	boolVar(flagSet, &edit, "edit")
	customVar(flagSet, &output, "o", "output")
	parseFlags(flagSet, args)

	r, path, err := openConfig()
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
		if _, err := readConfig(f); err != nil {
			return fmt.Errorf("not applying configuration updates because the file has a syntax error: %w", err)
		}
		if err := os.Rename(tmp, path); err != nil {
			return err
		}
	}

	config, err := loadConfig()
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
			r, _, err := openConfig()
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

type configuration struct {
	Registry struct {
		Location string `json:"location"`
	} `json:"registry"`
}

func defaultConfig() *configuration {
	c := new(configuration)
	c.Registry.Location = "~/.timecraft"
	return c
}

func openConfig() (io.ReadCloser, string, error) {
	path, err := configPath.Resolve()
	if err != nil {
		return nil, path, err
	}
	f, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, path, err
		}
		c := defaultConfig()
		b, _ := yaml.Marshal(c)
		return io.NopCloser(bytes.NewReader(b)), path, nil
	}
	return f, path, nil
}

func loadConfig() (*configuration, error) {
	r, _, err := openConfig()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return readConfig(r)
}

func readConfig(r io.Reader) (*configuration, error) {
	c := defaultConfig()
	d := yaml.NewDecoder(r)
	d.KnownFields(true)
	if err := d.Decode(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *configuration) createRegistry() (*timemachine.Registry, error) {
	p, err := human.Path(c.Registry.Location).Resolve()
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(filepath.Dir(p), 0777); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	return c.openRegistry()
}

func (c *configuration) openRegistry() (*timemachine.Registry, error) {
	p, err := human.Path(c.Registry.Location).Resolve()
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
