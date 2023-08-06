package tarfs

import (
	"io/fs"
	"syscall"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

// The placeholder type is an implementation of fileEntry used to represent file
// system entries that are not supported yet.
type placeholder struct {
	info sandbox.FileInfo
}

func (p *placeholder) open(fsys *fileSystem) (sandbox.File, error) {
	return nil, syscall.EPERM
}

func (p *placeholder) stat() sandbox.FileInfo {
	return p.info
}

func (p *placeholder) mode() fs.FileMode {
	return p.info.Mode
}
