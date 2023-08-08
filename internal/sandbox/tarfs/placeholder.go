package tarfs

import (
	"archive/tar"
	"io/fs"
	"syscall"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

// The placeholder type is an implementation of fileEntry used to represent file
// system entries that are not supported yet.
type placeholder struct {
	info sandbox.FileInfo
}

func newPlaceholder(header *tar.Header) *placeholder {
	info := header.FileInfo()
	mode := info.Mode()
	return &placeholder{
		info: sandbox.FileInfo{
			Size:  info.Size(),
			Mode:  mode.Type() | (mode.Perm() & 0555),
			Uid:   1,
			Gid:   1,
			Nlink: 1,
			Mtime: sandbox.TimeToTimespec(header.ModTime),
			Atime: sandbox.TimeToTimespec(header.AccessTime),
			Ctime: sandbox.TimeToTimespec(header.ChangeTime),
		},
	}
}

func (p *placeholder) open(fsys *FileSystem, name string) (sandbox.File, error) {
	return nil, syscall.EPERM
}

func (p *placeholder) stat() sandbox.FileInfo {
	return p.info
}

func (p *placeholder) mode() fs.FileMode {
	return p.info.Mode
}

func (p *placeholder) memsize() uintptr {
	return unsafe.Sizeof(placeholder{})
}
