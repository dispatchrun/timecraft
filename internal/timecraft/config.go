package timecraft

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"

	"github.com/stealthrocket/timecraft/internal/print/human"
	"gopkg.in/yaml.v3"
)

const (
	defaultConfigPath   = "~/.timecraft/config.yaml"
	defaultRegistryPath = "~/.timecraft/registry"
)

// Config is timecraft configuration.
type Config struct {
	Registry struct {
		Location Option[human.Path] `json:"location"`
	} `json:"registry"`
	Cache struct {
		Location Option[human.Path] `json:"location"`
	} `json:"cache"`
}

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
	c.Registry.Location = Some[human.Path](defaultRegistryPath)
	return c
}
