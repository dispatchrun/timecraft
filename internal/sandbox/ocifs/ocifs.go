package ocifs

import (
	"errors"
	"io/fs"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/fspath"
)

// FileSystem is an implementation of the sandbox.FileSystem interface that
// merges OCI layers into a single view.
type FileSystem struct {
	layers []sandbox.FileSystem
}

// New constructs a file system which combines layers into a flattened view
// which stacks layers on each other.
//
// The returned file system is read-only, it does not allow modifications of
// the layers that it was constructed from. This includes writes to files as
// well as modifcations of the directory structure.
//
// For the OCI layer specification, see:
// https://github.com/opencontainers/image-spec/blob/main/layer.md
func New(layers ...sandbox.FileSystem) *FileSystem {
	// Reverse the slice of layers so we can use range loops to iterate from
	// the upper to the lower layers.
	fsys := &FileSystem{
		layers: make([]sandbox.FileSystem, len(layers)),
	}
	for i, layer := range layers {
		fsys.layers[len(layers)-(i+1)] = layer
	}
	return fsys
}

// Open satisfies the sandbox.FileSystem interface.
func (fsys *FileSystem) Open(name string, flags int, mode fs.FileMode) (sandbox.File, error) {
	f, err := fsys.openRoot()
	if err != nil {
		return nil, err
	}
	if fspath.IsRoot(name) {
		return f, nil
	}
	defer f.Close()
	return f.Open(name, flags, mode)
}

func (fsys *FileSystem) openRoot() (sandbox.File, error) {
	if len(fsys.layers) == 0 {
		return nil, sandbox.ENOENT
	}

	files := make([]sandbox.File, 0, len(fsys.layers))
	defer func() {
		closeFiles(files)
	}()

	for _, layer := range fsys.layers {
		f, err := sandbox.OpenRoot(layer)
		if err != nil {
			if !errors.Is(err, sandbox.ENOENT) {
				return nil, err
			}
		} else {
			files = append(files, f)
		}
	}

	root := &file{
		fsys:   fsys,
		layers: &fileLayers{files: files},
	}
	ref(root.layers)
	files = nil
	return root, nil
}
