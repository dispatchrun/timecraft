package tarfs

import (
	"io"
	"io/fs"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

type file struct {
	info   sandbox.FileInfo
	offset int64
}

func (f *file) open(fsys *FileSystem, name string) (sandbox.File, error) {
	open := &openFile{name: name}
	open.file.Store(f)
	open.data = *io.NewSectionReader(fsys.data, f.offset, f.info.Size)
	return open, nil
}

func (f *file) stat() sandbox.FileInfo {
	return f.info
}

func (f *file) mode() fs.FileMode {
	return f.info.Mode
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
		return file.info, nil
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
