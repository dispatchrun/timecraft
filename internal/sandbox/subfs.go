package sandbox

import (
	"io/fs"
	"path"
	"sync"

	"github.com/stealthrocket/timecraft/internal/sandbox/fspath"
)

// SubFS constructs a FileSystem which exposes a base file system at a given
// directory path.
//
// The path to the base file system is read-only, it cannot be modified using
// operations that mutate the directory structure (e.g. rename, rmdir, etc...).
//
// Using a file system at a sub-path can be useful when combining layers using
// the ocifs pacakge; it enables the application to mount sub file systems at
// custom locations.
func SubFS(base FileSystem, dir string) FileSystem {
	dir = path.Join("/", dir)
	dir = dir[1:] // trim the leading slash
	if dir == "" {
		return base
	}
	return newSubFS(base, dir)
}

func newSubFS(base FileSystem, dir string) *subFS {
	name, path := fspath.Walk(dir)

	fsys := &subFS{name: name}
	fsys.prev = fsys // assume root, gets overriden below if not

	if path == "" {
		fsys.next = base
	} else {
		sub := newSubFS(base, path)
		sub.prev = fsys
		fsys.next = sub
	}

	return fsys
}

type subFS struct {
	prev *subFS
	name string
	next FileSystem
}

func (fsys *subFS) Open(name string, flags OpenFlags, mode fs.FileMode) (File, error) {
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

func (fsys *subFS) openRoot() (File, error) {
	return &subFile{base: &subDir{fsys: fsys}}, nil
}

type subFile struct {
	prev *subFile
	base File
}

func (f *subFile) Fd() uintptr {
	return f.base.Fd()
}

func (f *subFile) Close() error {
	return f.base.Close()
}

func (f *subFile) openRoot() (File, error) {
	for f.prev != nil {
		f = f.prev
	}
	// Here we have the guarantee that f.base is of type *subDir because we
	// walked the chain of directories all the way back to the top.
	return &subFile{base: &subDir{fsys: f.base.(*subDir).fsys}}, nil
}

func (f *subFile) openSelf() (File, error) {
	file := &subFile{prev: f.prev}
	switch base := f.base.(type) {
	case *subDir:
		file.base = &subDir{fsys: base.fsys}
	default:
		base, err := openSelf(f.base)
		if err != nil {
			return nil, err
		}
		file.base = base
	}
	return file, nil
}

func (f *subFile) openParent() (File, error) {
	if f.prev == nil {
		return f.openRoot()
	}
	prev := f.prev
	file := &subFile{prev: prev.prev}
	switch base := prev.base.(type) {
	case *subDir:
		file.base = &subDir{fsys: base.fsys}
	default:
		base, err := openParent(f.base)
		if err != nil {
			return nil, err
		}
		file.base = base
	}
	return file, nil
}

func (f *subFile) openFile(name string, flags OpenFlags, mode fs.FileMode) (File, error) {
	base, err := f.base.Open(name, flags, mode)
	if err != nil {
		return nil, err
	}
	return &subFile{prev: f, base: base}, nil
}

func (f *subFile) Open(name string, flags OpenFlags, mode fs.FileMode) (File, error) {
	return FileOpen(f, name, flags, ^OpenFlags(0), mode,
		(*subFile).openRoot,
		(*subFile).openSelf,
		(*subFile).openParent,
		(*subFile).openFile,
	)
}

func (f *subFile) Readv(iovs [][]byte) (int, error) {
	return f.base.Readv(iovs)
}

func (f *subFile) Writev(iovs [][]byte) (int, error) {
	return f.base.Writev(iovs)
}

func (f *subFile) Preadv(iovs [][]byte, offset int64) (int, error) {
	return f.base.Preadv(iovs, offset)
}

func (f *subFile) Pwritev(iovs [][]byte, offset int64) (int, error) {
	return f.base.Pwritev(iovs, offset)
}

func (f *subFile) CopyRange(srcOffset int64, dst File, dstOffset int64, length int) (int, error) {
	return f.base.CopyRange(srcOffset, dst, dstOffset, length)
}

func (f *subFile) Seek(offset int64, whence int) (int64, error) {
	return f.base.Seek(offset, whence)
}

func (f *subFile) Allocate(offset, length int64) error {
	return f.base.Allocate(offset, length)
}

func (f *subFile) Truncate(size int64) error {
	return f.base.Truncate(size)
}

func (f *subFile) Sync() error {
	return f.base.Sync()
}

func (f *subFile) Datasync() error {
	return f.base.Datasync()
}

func (f *subFile) Flags() (OpenFlags, error) {
	return f.base.Flags()
}

func (f *subFile) SetFlags(flags OpenFlags) error {
	return f.base.SetFlags(flags)
}

func (f *subFile) ReadDirent(buf []byte) (int, error) {
	return f.base.ReadDirent(buf)
}

func (f *subFile) Stat(name string, flags LookupFlags) (FileInfo, error) {
	return FileStat(f, name, flags, func(at *subFile, name string) (FileInfo, error) {
		return at.base.Stat(name, AT_SYMLINK_NOFOLLOW)
	})
}

func (f *subFile) Readlink(name string, buf []byte) (int, error) {
	return FileReadlink(f, name, func(at *subFile, name string) (int, error) {
		return at.base.Readlink(name, buf)
	})
}

func (f *subFile) Chtimes(name string, times [2]Timespec, flags LookupFlags) error {
	return f.resolvePath(name, flags, func(at *subFile, name string) error {
		return at.base.Chtimes(name, times, AT_SYMLINK_NOFOLLOW)
	})
}

func (f *subFile) Mkdir(name string, mode fs.FileMode) error {
	return f.resolvePath(name, 0, func(at *subFile, name string) error {
		return at.base.Mkdir(name, mode)
	})
}

func (f *subFile) Rmdir(name string) error {
	return f.resolvePath(name, 0, func(at *subFile, name string) error {
		return at.base.Rmdir(name)
	})
}

func (f *subFile) Rename(oldName string, newDir File, newName string, flags RenameFlags) error {
	d, ok := newDir.(*subFile)
	if !ok {
		return EXDEV
	}
	return f.resolvePath(oldName, 0, func(f1 *subFile, name1 string) error {
		return d.resolvePath(newName, 0, func(f2 *subFile, name2 string) error {
			return f1.base.Rename(name1, f2.base, name2, flags)
		})
	})
}

func (f *subFile) Link(oldName string, newDir File, newName string, flags LookupFlags) error {
	d, ok := newDir.(*subFile)
	if !ok {
		return EXDEV
	}
	return f.resolvePath(oldName, 0, func(f1 *subFile, name1 string) error {
		return d.resolvePath(newName, 0, func(f2 *subFile, name2 string) error {
			return f1.base.Link(name1, f2.base, name2, AT_SYMLINK_NOFOLLOW)
		})
	})
}

func (f *subFile) Symlink(oldName, newName string) error {
	return f.resolvePath(newName, 0, func(at *subFile, name string) error {
		return at.base.Symlink(oldName, name)
	})
}

func (f *subFile) Unlink(name string) error {
	return f.resolvePath(name, 0, func(at *subFile, name string) error {
		return at.base.Unlink(name)
	})
}

func (f *subFile) resolvePath(name string, flags LookupFlags, do func(*subFile, string) error) error {
	_, err := ResolvePath(f, name, flags, func(at *subFile, name string) (_ struct{}, err error) {
		err = do(at, name)
		return
	})
	return err
}

type subDir struct {
	mutex  sync.Mutex
	seek   uint64
	offset uint64
	fsys   *subFS
}

func (d *subDir) Fd() uintptr {
	return ^uintptr(0)
}

func (d *subDir) Close() error {
	return nil
}

func (d *subDir) Open(name string, flags OpenFlags, mode fs.FileMode) (File, error) {
	switch name {
	case ".":
		return &subDir{fsys: d.fsys}, nil
	case "..":
		return &subDir{fsys: d.fsys.prev}, nil
	case d.fsys.name:
		if sub, ok := d.fsys.next.(*subFS); ok {
			return &subDir{fsys: sub}, nil
		}
		return OpenRoot(d.fsys.next)
	default:
		return nil, ENOENT
	}
}

func (d *subDir) Readv([][]byte) (int, error) {
	return 0, EISDIR
}

func (d *subDir) Writev([][]byte) (int, error) {
	return 0, EISDIR
}

func (d *subDir) Preadv([][]byte, int64) (int, error) {
	return 0, EISDIR
}

func (d *subDir) Pwritev([][]byte, int64) (int, error) {
	return 0, EISDIR
}

func (d *subDir) CopyRange(int64, File, int64, int) (int, error) {
	return 0, EISDIR
}

func (d *subDir) Seek(offset int64, whence int) (int64, error) {
	if offset != 0 || whence != 0 {
		return 0, EINVAL
	}
	d.mutex.Lock()
	d.seek = 0
	d.mutex.Unlock()
	return 0, nil
}

func (d *subDir) Allocate(int64, int64) error {
	return EISDIR
}

func (d *subDir) Truncate(int64) error {
	return EISDIR
}

func (d *subDir) Sync() error {
	return nil
}

func (d *subDir) Datasync() error {
	return nil
}

func (d *subDir) Flags() (OpenFlags, error) {
	return 0, nil
}

func (d *subDir) SetFlags(OpenFlags) error {
	return EPERM
}

func (d *subDir) ReadDirent(buf []byte) (n int, err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	for i, name := range [...]string{".", "..", d.fsys.name} {
		if uint64(i) != d.seek {
			continue
		}
		wn := WriteDirent(buf[n:], fs.ModeDir, 0, d.offset, name)
		n += wn
		if n == len(buf) {
			break
		}
		d.seek++
		d.offset += uint64(wn)
	}

	return n, nil
}

func (d *subDir) Stat(name string, flags LookupFlags) (info FileInfo, err error) {
	// This method is intended to be called from subFile.Stat only, so it
	// should never have to follow symlinks.
	if (flags & AT_SYMLINK_NOFOLLOW) == 0 {
		return info, ENOSYS
	}
	switch name {
	case "", d.fsys.name:
		// TODO:
		// - link count should be the number of reference to the directory
		// - access and modify time should not be zero (but what?)
		info.Nlink = 1
		info.Mode = fs.ModeDir | 0755
		return info, nil
	default:
		return info, ENOENT
	}
}

func (d *subDir) Readlink(name string, buf []byte) (int, error) {
	switch name {
	case ".", "..", d.fsys.name:
		return 0, EINVAL
	default:
		return 0, ENOENT
	}
}

func (d *subDir) Chtimes(string, [2]Timespec, LookupFlags) error {
	return EBUSY
}

func (d *subDir) Mkdir(string, fs.FileMode) error {
	return EBUSY
}

func (d *subDir) Rmdir(string) error {
	return EBUSY
}

func (d *subDir) Rename(string, File, string, RenameFlags) error {
	return EBUSY
}

func (d *subDir) Link(string, File, string, LookupFlags) error {
	return EBUSY
}

func (d *subDir) Symlink(string, string) error {
	return EBUSY
}

func (d *subDir) Unlink(string) error {
	return EBUSY
}
