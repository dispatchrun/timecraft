package tarfs

import (
	"io/fs"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

type symlink struct {
	link string
	info sandbox.FileInfo
}

func (s *symlink) open(fsys *FileSystem, name string) (sandbox.File, error) {
	return nil, sandbox.ELOOP
}

func (s *symlink) stat() sandbox.FileInfo {
	return s.info
}

func (s *symlink) mode() fs.FileMode {
	return s.info.Mode
}

func (s *symlink) memsize() uintptr {
	return unsafe.Sizeof(symlink{}) + uintptr(len(s.link))
}
