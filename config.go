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
	"github.com/tetratelabs/wazero"
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

type nullable[T any] struct {
	value T
	exist bool
}

func null[T any]() nullable[T] {
	return nullable[T]{exist: false}
}

func value[T any](v T) nullable[T] {
	return nullable[T]{value: v, exist: true}
}

func (v *nullable[T]) Value() (T, bool) {
	return v.value, v.exist
}

func (v *nullable[T]) MarshalJSON() ([]byte, error) {
	if !v.exist {
		return []byte("null"), nil
	}
	return json.Marshal(v.value)
}

func (v *nullable[T]) MarshalYAML() (any, error) {
	if !v.exist {
		return nil, nil
	}
	return v.value, nil
}

func (v *nullable[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		v.exist = false
		return nil
	} else if err := json.Unmarshal(b, &v.value); err != nil {
		v.exist = false
		return err
	} else {
		v.exist = true
		return nil
	}
}

func (v *nullable[T]) UnmarshalYAML(node *yaml.Node) error {
	if node.Value == "" || node.Value == "~" || node.Value == "null" {
		v.exist = false
		return nil
	} else if err := node.Decode(&v.value); err != nil {
		v.exist = false
		return err
	} else {
		v.exist = true
		return nil
	}
}

type configuration struct {
	Registry struct {
		Location nullable[human.Path] `json:"location"`
	} `json:"registry"`
	Cache struct {
		Location nullable[human.Path] `json:"location"`
	} `json:"cache"`
}

func defaultConfig() *configuration {
	c := new(configuration)
	c.Registry.Location = value[human.Path]("~/.timecraft/registry")
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

func (c *configuration) newRuntime(ctx context.Context) wazero.Runtime {
	config := wazero.NewRuntimeConfig()

	if cachePath, ok := c.Cache.Location.Value(); ok {
		// The cache is an optimization, so if we encounter errors we notify the
		// user but still go ahead with the runtime instantiation.
		path, err := cachePath.Resolve()
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: resolving timecraft cache location: %s\n", err)
		} else {
			cache, err := createCacheDirectory(path)
			if err != nil {
				fmt.Fprintf(os.Stderr, "ERR: creating timecraft cache directory: %s\n", err)
			} else {
				config = config.WithCompilationCache(cache)
			}
		}
	}

	return wazero.NewRuntimeWithConfig(ctx, config)
}

func (c *configuration) createRegistry() (*timemachine.Registry, error) {
	location, ok := c.Registry.Location.Value()
	if ok {
		path, err := location.Resolve()
		if err != nil {
			return nil, err
		}
		if err := createDirectory(path); err != nil {
			return nil, err
		}
	}
	return c.openRegistry()
}

func (c *configuration) openRegistry() (*timemachine.Registry, error) {
	store := object.EmptyStore()
	location, ok := c.Registry.Location.Value()
	if ok {
		path, err := location.Resolve()
		if err != nil {
			return nil, err
		}
		dir, err := object.DirStore(path)
		if err != nil {
			return nil, err
		}
		store = dir
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

func createDirectory(path string) error {
	if err := os.MkdirAll(path, 0777); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return err
		}
	}
	return nil
}

func createCacheDirectory(path string) (wazero.CompilationCache, error) {
	if err := createDirectory(path); err != nil {
		return nil, err
	}
	return wazero.NewCompilationCacheWithDir(path)
}
