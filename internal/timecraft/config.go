package timecraft

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/tetratelabs/wazero"
	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath   = "~/.timecraft/config.yaml"
	defaultRegistryPath = "~/.timecraft/registry"
)

// ConfigPath is the path to the timecraft configuration.
var ConfigPath human.Path = defaultConfigPath

// LoadConfig opens and reads the configuration file.
func LoadConfig() (*Config, error) {
	r, _, err := OpenConfig()
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return ReadConfig(r)
}

// OpenConfig opens the configuration file.
func OpenConfig() (io.ReadCloser, string, error) {
	path, err := ConfigPath.Resolve()
	if err != nil {
		return nil, path, err
	}
	f, err := os.Open(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, path, err
		}
		c := DefaultConfig()
		b, _ := yaml.Marshal(c)
		return io.NopCloser(bytes.NewReader(b)), path, nil
	}
	return f, path, nil
}

// ReadConfig reads and parses configuration.
func ReadConfig(r io.Reader) (*Config, error) {
	c := DefaultConfig()
	d := yaml.NewDecoder(r)
	d.KnownFields(true)
	if err := d.Decode(c); err != nil {
		return nil, err
	}
	return c, nil
}

// DefaultConfig is the default configuration.
func DefaultConfig() *Config {
	c := new(Config)
	c.Registry.Location = NullableValue[human.Path](defaultRegistryPath)
	return c
}

// Config is timecraft configuration.
type Config struct {
	Registry struct {
		Location Nullable[human.Path] `json:"location"`
	} `json:"registry"`
	Cache struct {
		Location Nullable[human.Path] `json:"location"`
	} `json:"cache"`
}

// NewRuntime constructs a wazero.Runtime that's configured according
// to Config.
func (c *Config) NewRuntime(ctx context.Context) (wazero.Runtime, error) {
	config := wazero.NewRuntimeConfig()

	var cache wazero.CompilationCache
	if cachePath, ok := c.Cache.Location.Value(); ok {
		// The cache is an optimization, so if we encounter errors we notify the
		// user but still go ahead with the runtime instantiation.
		path, err := cachePath.Resolve()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve timecraft cache location: %w", err)
		} else {
			cache, err = createCacheDirectory(path)
			if err != nil {
				return nil, fmt.Errorf("failed to create timecraft cache directory: %w", err)
			} else {
				config = config.WithCompilationCache(cache)
			}
		}
	}

	runtime := wazero.NewRuntimeWithConfig(ctx, config)
	if cache != nil {
		runtime = &runtimeWithCompilationCache{
			Runtime: runtime,
			cache:   cache,
		}
	}
	return runtime, nil
}

type runtimeWithCompilationCache struct {
	wazero.Runtime
	cache wazero.CompilationCache
}

func (r *runtimeWithCompilationCache) Close(ctx context.Context) error {
	if r.cache != nil {
		defer r.cache.Close(ctx)
	}
	return r.Runtime.Close(ctx)
}

// CreateRegistry creates a timemachine registry.
func (c *Config) CreateRegistry() (*timemachine.Registry, error) {
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
	return c.OpenRegistry()
}

// OpenRegistry opens a timemachine registry.
func (c *Config) OpenRegistry() (*timemachine.Registry, error) {
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

type Nullable[T any] struct {
	value T
	exist bool
}

// Note: commented to satisfy the linter, uncomment if we need it
//
// func null[T any]() Nullable[T] {
// 	return Nullable[T]{exist: false}
// }

func NullableValue[T any](v T) Nullable[T] {
	return Nullable[T]{value: v, exist: true}
}

func (v Nullable[T]) Value() (T, bool) {
	return v.value, v.exist
}

func (v Nullable[T]) MarshalJSON() ([]byte, error) {
	if !v.exist {
		return []byte("null"), nil
	}
	return json.Marshal(v.value)
}

func (v Nullable[T]) MarshalYAML() (any, error) {
	if !v.exist {
		return nil, nil
	}
	return v.value, nil
}

func (v *Nullable[T]) UnmarshalJSON(b []byte) error {
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

func (v *Nullable[T]) UnmarshalYAML(node *yaml.Node) error {
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
