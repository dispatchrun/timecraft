package tarfs

import (
	"archive/tar"
	"io/fs"
	"time"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

type symlink struct {
	perm  fs.FileMode
	nlink uint32
	mtime int64
	atime int64
	ctime int64
	link  string
}

func newSymlink(header *tar.Header) *symlink {
	mode := header.FileInfo().Mode()
	return &symlink{
		perm:  mode.Perm() & 0555,
		nlink: 1,
		mtime: header.ModTime.UnixNano(),
		atime: header.AccessTime.UnixNano(),
		ctime: header.ChangeTime.UnixNano(),
		link:  header.Linkname,
	}
}

func (s *symlink) open(fsys *FileSystem) (sandbox.File, error) {
	return nil, sandbox.ELOOP
}

func (s *symlink) stat() sandbox.FileInfo {
	return sandbox.FileInfo{
		Mode:  s.mode(),
		Uid:   1,
		Gid:   1,
		Nlink: uint64(s.nlink),
		Mtime: sandbox.TimeToTimespec(time.Unix(0, s.mtime)),
		Atime: sandbox.TimeToTimespec(time.Unix(0, s.atime)),
		Ctime: sandbox.TimeToTimespec(time.Unix(0, s.ctime)),
	}
}

func (s *symlink) mode() fs.FileMode {
	return fs.ModeSymlink | s.perm
}

func (s *symlink) memsize() uintptr {
	return unsafe.Sizeof(symlink{}) + uintptr(len(s.link))
}
