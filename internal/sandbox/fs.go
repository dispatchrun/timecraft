package sandbox

import (
	"errors"
	"io/fs"
	"path"
	"time"
)

const (
	MaxFollowSymlink = 10
)

type FileSystem interface {
	Open(name string, flags int, mode fs.FileMode) (File, error)
}

func Open(fsys FileSystem, name string) (File, error) {
	return fsys.Open(name, O_RDONLY, 0)
}

func OpenDir(fsys FileSystem, name string) (File, error) {
	return fsys.Open(name, O_DIRECTORY, 0)
}

func OpenRoot(fsys FileSystem) (File, error) {
	return OpenDir(fsys, "/")
}

func Lstat(fsys FileSystem, name string) (fs.FileInfo, error) {
	info, err := atRoot(fsys, func(dir File) (fs.FileInfo, error) {
		return dir.Lstat(name)
	})
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: unwrapPathError(err)}
	}
	return info, nil
}

func Stat(fsys FileSystem, name string) (fs.FileInfo, error) {
	info, err := atRoot(fsys, func(dir File) (fs.FileInfo, error) {
		for i := 0; i < MaxFollowSymlink; i++ {
			stat, err := dir.Lstat(name)
			if err != nil {
				return nil, err
			}
			if stat.Mode().Type() != fs.ModeSymlink {
				return stat, nil
			}
			link, err := dir.Readlink(name)
			if err != nil {
				return nil, err
			}
			if !path.IsAbs(link) {
				link = path.Join(name, link)
			}
			name = link
		}
		return nil, ELOOP
	})
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: unwrapPathError(err)}
	}
	return info, nil
}

func atRoot[F func(File) (R, error), R any](fsys FileSystem, do F) (ret R, err error) {
	d, err := OpenRoot(fsys)
	if err != nil {
		return ret, err
	}
	defer d.Close()
	return do(d)
}

type File interface {
	Fd() uintptr

	Name() string

	Close() error

	Open(name string, flags int, mode fs.FileMode) (File, error)

	Read(data []byte) (int, error)

	ReadAt(data []byte, offset int64) (int, error)

	Write(data []byte) (int, error)

	WriteAt(data []byte, offset int64) (int, error)

	Seek(offset int64, whence int) (int64, error)

	Stat() (fs.FileInfo, error)

	Sync() error

	Datasync() error

	Truncate(size int64) error

	SetFlags(flags int) error

	ReadDir(n int) ([]fs.DirEntry, error)

	Lstat(name string) (fs.FileInfo, error)

	Readlink(name string) (string, error)

	Chtimes(name string, atime, mtime time.Time) error

	Mkdir(name string, mode uint32) error

	Rmdir(name string) error

	Rename(oldName string, newDir File, newName string) error

	Link(oldName string, newDir File, newName string) error

	Symlink(oldName, newName string) error

	Unlink(name string) error
}

func FS(fsys FileSystem) fs.FS {
	return &fsFileSystem{fsys}
}

type fsFileSystem struct{ base FileSystem }

func (fsys *fsFileSystem) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, fsError("open", name, fs.ErrNotExist)
	}
	f, err := Open(fsys.base, name)
	if err != nil {
		return nil, fsError("open", name, err)
	}
	return &fsFile{fsys.base, f}, nil
}

func (fsys *fsFileSystem) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, fsError("stat", name, fs.ErrNotExist)
	}
	s, err := Stat(fsys.base, name)
	if err != nil {
		return nil, fsError("stat", name, err)
	}
	return s, nil
}

func fsError(op, name string, err error) error {
	err = unwrapPathError(err)
	switch {
	case errors.Is(err, EEXIST):
		err = fs.ErrExist
	case errors.Is(err, ENOENT):
		err = fs.ErrNotExist
	case errors.Is(err, EINVAL):
		err = fs.ErrInvalid
	case errors.Is(err, EBADF):
		err = fs.ErrClosed
	}
	return &fs.PathError{Op: op, Path: name, Err: err}
}

func unwrapPathError(err error) error {
	e, ok := err.(*fs.PathError)
	if ok {
		return e.Err
	}
	return err
}

var (
	_ fs.StatFS = (*fsFileSystem)(nil)
)

type fsFile struct {
	fsys FileSystem
	File
}

func (f *fsFile) ReadDir(n int) ([]fs.DirEntry, error) {
	dirents, err := f.File.ReadDir(n)
	if len(dirents) > 0 {
		fsDirEntries := make([]fsDirEntry, len(dirents))
		for i, dirent := range dirents {
			fsDirEntries[i] = fsDirEntry{f, dirent}
		}
		for i := range dirents {
			dirents[i] = &fsDirEntries[i]
		}
	}
	return dirents, err
}

type fsDirEntry struct {
	file *fsFile
	fs.DirEntry
}

func (dirent *fsDirEntry) Info() (fs.FileInfo, error) {
	s, err := dirent.file.Lstat(dirent.Name())
	if err != nil {
		return nil, fsError("stat", dirent.Name(), err)
	}
	return s, nil
}
