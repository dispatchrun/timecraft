package sandbox

import (
	"errors"
	"io"
	"io/fs"
	"strings"
	"time"
)

const (
	// maxFollowSymlink is the hardcoded limit of symbolic links that may be
	// followed when resolving paths.
	//
	// This limit applies to RootFS, EvalSymlinks, and the functions that
	// depend on it.
	maxFollowSymlink = 10
)

// FileSystem is the interface representing file systems.
//
// The interface has a single method used to open a file at a path on the file
// system, which may be a directory. Often time this method is used to open the
// root directory and use the methods of the returned File instance to access
// the rest of the directory tree.
type FileSystem interface {
	Open(name string, flags int, mode fs.FileMode) (File, error)
}

func Create(fsys FileSystem, name string, mode fs.FileMode) (File, error) {
	return fsys.Open(name, O_CREAT|O_TRUNC|O_WRONLY, mode)
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

func EvalSymlinks(fsys FileSystem, name string) (string, error) {
	return withRoot2(fsys, func(dir File) (string, error) { return evalSymlinks(dir, name) })
}

func evalSymlinks(dir File, name string) (string, error) {
	path := name

	for i := 0; i < maxFollowSymlink; i++ {
		link, err := dir.Readlink(path)
		if err != nil {
			if errors.Is(err, EINVAL) {
				return path, nil
			}
			return "", err
		}
		path = link
	}

	return "", &fs.PathError{Op: "readlink", Path: name, Err: ELOOP}
}

func Lstat(fsys FileSystem, name string) (fs.FileInfo, error) {
	info, err := withRoot2(fsys, func(dir File) (fs.FileInfo, error) {
		return dir.Lstat(name)
	})
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: unwrap(err)}
	}
	return info, nil
}

func Stat(fsys FileSystem, name string) (fs.FileInfo, error) {
	return withRoot2(fsys, func(dir File) (fs.FileInfo, error) {
		path, err := evalSymlinks(dir, name)
		if err != nil {
			return nil, &fs.PathError{Op: "stat", Path: name, Err: unwrap(err)}
		}
		stat, err := dir.Lstat(path)
		if err != nil {
			return nil, err
		}
		if stat.Mode().Type() == fs.ModeSymlink {
			// If the file system was modified concurrently, the resolved target
			// of symbolic links may have been replaced by a symbolic link that
			// we did not follow. Since this is a race condition, we prefer
			// returning an error rather than attempt to handle this condition.
			return nil, &fs.PathError{Op: "stat", Path: name, Err: ELOOP}
		}
		return stat, nil
	})
}

func ReadFile(fsys FileSystem, name string, flags int) ([]byte, error) {
	f, err := fsys.Open(name, flags|O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	b := make([]byte, s.Size())
	n, err := io.ReadFull(f, b)
	return b[:n], err
}

func WriteFile(fsys FileSystem, name string, data []byte, mode fs.FileMode) error {
	f, err := fsys.Open(name, O_CREAT|O_WRONLY|O_TRUNC|O_EXCL, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func MkdirAll(fsys FileSystem, name string, mode fs.FileMode) error {
	if err := mkdirAll(fsys, name, mode); err != nil {
		return &fs.PathError{Op: "mkdir", Path: name, Err: unwrap(err)}
	}
	return nil
}

func mkdirAll(fsys FileSystem, name string, mode fs.FileMode) error {
	path := cleanPath(name)
	if path == "/" || path == "." {
		return nil
	}
	path = strings.TrimPrefix(path, "/")

	d, err := OpenRoot(fsys)
	if err != nil {
		return err
	}
	defer func() { d.Close() }()

	for path != "" {
		var dir string
		dir, path = walkPath(path)
		if dir == "." {
			dir, path = path, ""
		}

		if err := d.Mkdir(dir, mode); err != nil {
			if !errors.Is(err, EEXIST) {
				return err
			}
		}

		f, err := d.Open(dir, O_DIRECTORY|O_NOFOLLOW, 0)
		if err != nil {
			return err
		}
		d.Close()
		d = f
	}
	return nil
}

func Mkdir(fsys FileSystem, name string, mode fs.FileMode) error {
	return withRoot1(fsys, func(dir File) error { return dir.Mkdir(name, mode) })
}

func Rmdir(fsys FileSystem, name string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Rmdir(name) })
}

func Link(fsys FileSystem, oldName, newName string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Link(oldName, dir, newName) })
}

func Symlink(fsys FileSystem, oldName, newName string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Symlink(oldName, newName) })
}

func Readlink(fsys FileSystem, name string) (string, error) {
	return withRoot2(fsys, func(dir File) (string, error) { return dir.Readlink(name) })
}

func Unlink(fsys FileSystem, name string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Unlink(name) })
}

func Rename(fsys FileSystem, oldName, newName string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Rename(oldName, dir, newName) })
}

func withRoot1(fsys FileSystem, do func(File) error) error {
	d, err := OpenRoot(fsys)
	if err != nil {
		return err
	}
	defer d.Close()
	return do(d)
}

func withRoot2[R any](fsys FileSystem, do func(File) (R, error)) (ret R, err error) {
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

	Flags() (int, error)

	SetFlags(flags int) error

	ReadDir(n int) ([]fs.DirEntry, error)

	Lstat(name string) (fs.FileInfo, error)

	Readlink(name string) (string, error)

	Chtimes(name string, atime, mtime time.Time) error

	Mkdir(name string, mode fs.FileMode) error

	Rmdir(name string) error

	Rename(oldName string, newDir File, newName string) error

	Link(oldName string, newDir File, newName string) error

	Symlink(oldName, newName string) error

	Unlink(name string) error
}

// FS constructs a fs.FS backed by a FileSystem instance.
//
// The returned fs.FS implements fs.StatFS.
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
	err = unwrap(err)
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

func unwrap(err error) error {
	if e := errors.Unwrap(err); e != nil {
		err = e
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
