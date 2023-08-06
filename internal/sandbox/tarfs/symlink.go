package tarfs

import (
	"io/fs"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

type symlink struct {
	name string
	link string
	info sandbox.FileInfo
}

func (s *symlink) open(fsys *fileSystem) (sandbox.File, error) {
	return nil, sandbox.ELOOP
}

func (s *symlink) stat() sandbox.FileInfo {
	return s.info
}

func (s *symlink) mode() fs.FileMode {
	return s.info.Mode
}
