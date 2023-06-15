package timecraft

import (
	"errors"
	"io/fs"
	"os"

	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

// CreateRegistry creates a timemachine registry.
func CreateRegistry(config *Config) (*timemachine.Registry, error) {
	location, ok := config.Registry.Location.Value()
	if ok {
		path, err := location.Resolve()
		if err != nil {
			return nil, err
		}
		if err := createDirectory(path); err != nil {
			return nil, err
		}
	}
	return OpenRegistry(config)
}

// OpenRegistry opens a timemachine registry.
func OpenRegistry(config *Config) (*timemachine.Registry, error) {
	store := object.EmptyStore()
	location, ok := config.Registry.Location.Value()
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
