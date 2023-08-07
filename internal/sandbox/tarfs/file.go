package tarfs

import (
	"archive/tar"
	"io"
	"io/fs"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

type file struct {
	perm   fs.FileMode
	nlink  uint32
	size   int64
	mtime  int64
	atime  int64
	ctime  int64
	offset int64
}

func newFile(header *tar.Header, offset int64) *file {
	info := header.FileInfo()
	mode := info.Mode()
	return &file{
		perm:   mode.Perm() & 0555,
		nlink:  1,
		size:   info.Size(),
		mtime:  header.ModTime.UnixNano(),
		atime:  header.AccessTime.UnixNano(),
		ctime:  header.ChangeTime.UnixNano(),
		offset: offset,
	}
}

func (f *file) open(fsys *FileSystem, name string) (sandbox.File, error) {
	open := &openFile{name: name}
	open.file.Store(f)
	open.data = *io.NewSectionReader(fsys.data, f.offset, f.size)
	return open, nil
}

func (f *file) stat() sandbox.FileInfo {
	return sandbox.FileInfo{
		Mode:  f.mode(),
		Uid:   1,
		Gid:   1,
		Nlink: uint64(f.nlink),
		Size:  f.size,
		Mtime: sandbox.TimeToTimespec(time.Unix(0, f.mtime)),
		Atime: sandbox.TimeToTimespec(time.Unix(0, f.atime)),
		Ctime: sandbox.TimeToTimespec(time.Unix(0, f.ctime)),
	}
}

func (f *file) mode() fs.FileMode {
	return f.perm
}

func (f *file) memsize() uintptr {
	return unsafe.Sizeof(file{})
}

type openFile struct {
	leafFile
	name string
	file atomic.Pointer[file]
	seek sync.Mutex
	data io.SectionReader
}

func (f *openFile) Name() string {
	return f.name
}

func (f *openFile) Close() error {
	f.file.Store(nil)
	return nil
}

func (f *openFile) Stat(name string, flags int) (sandbox.FileInfo, error) {
	file := f.file.Load()
	switch {
	case file == nil:
		return sandbox.FileInfo{}, sandbox.EBADF
	case name != "":
		return sandbox.FileInfo{}, sandbox.ENOTDIR
	default:
		return file.stat(), nil
	}
}

func (f *openFile) Readlink(name string, buf []byte) (int, error) {
	switch {
	case f.file.Load() == nil:
		return 0, sandbox.EBADF
	case name != "":
		return 0, sandbox.ENOTDIR
	default:
		return 0, sandbox.EINVAL
	}
}

func (f *openFile) Readv(buf [][]byte) (int, error) {
	if f.file.Load() == nil {
		return 0, sandbox.EBADF
	}
	read := 0

	f.seek.Lock() // synchronize modifications of the seek offset
	defer f.seek.Unlock()

	for _, b := range buf {
		n, err := f.data.Read(b)
		read += n
		if err != nil {
			if err == io.EOF {
				break
			}
			return read, err
		}
	}

	return read, nil
}

func (f *openFile) Preadv(buf [][]byte, off int64) (int, error) {
	if f.file.Load() == nil {
		return 0, sandbox.EBADF
	}
	read := 0

	for _, b := range buf {
		n, err := f.data.ReadAt(b, off)
		read += n
		off += int64(n)
		if err != nil {
			if err == io.EOF {
				break
			}
			return read, err
		}
	}

	return read, nil
}

func (f *openFile) Seek(offset int64, whence int) (int64, error) {
	if f.file.Load() == nil {
		return 0, sandbox.EBADF
	}
	f.seek.Lock()
	defer f.seek.Unlock()
	return f.data.Seek(offset, whence)
}
