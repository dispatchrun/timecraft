package overlayfs

import (
	"io/fs"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/fspath"
)

type FileSystem struct {
	lower sandbox.FileSystem
	upper sandbox.FileSystem
}

func New(lower, upper sandbox.FileSystem) *FileSystem {
	return &FileSystem{lower: lower, upper: upper}
}

func (fsys *FileSystem) Open(name string, flags sandbox.OpenFlags, mode fs.FileMode) (sandbox.File, error) {
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
	lower, err := sandbox.OpenRoot(fsys.lower)
	if err != nil {
		return nil, err
	}
	upper, err := sandbox.OpenRoot(fsys.upper)
	if err != nil {
		lower.Close()
		return nil, err
	}
	return newFile(newFileOverlay(nil, lower, upper, "/")), nil
}
