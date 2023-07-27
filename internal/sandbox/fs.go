package sandbox

import (
	"errors"
	"io/fs"
	"path"
	"path/filepath"
	"strings"
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
	info, err := withRoot(fsys, func(dir File) (fs.FileInfo, error) {
		return dir.Lstat(name)
	})
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: unwrapPathError(err)}
	}
	return info, nil
}

func Stat(fsys FileSystem, name string) (fs.FileInfo, error) {
	info, err := withRoot(fsys, func(dir File) (fs.FileInfo, error) {
		for i := 0; i < MaxFollowSymlink; i++ {
			stat, err := dir.Lstat(name)
			if err != nil {
				return nil, err
			}
			if stat.Mode().Type() != fs.ModeSymlink {
				return stat, nil
			}
			link, err := dir.ReadLink(name)
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

func withRoot[F func(File) (R, error), R any](fsys FileSystem, do F) (ret R, err error) {
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

	ReadDir(n int) ([]fs.DirEntry, error)

	Lstat(name string) (fs.FileInfo, error)

	ReadLink(name string) (string, error)

	SetFlags(flags int) error
}

func DirFS(path string) (FileSystem, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return dirFS(path), nil
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
		err = fsError("stat", name, err)
	}
	return s, err
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

func RootFS(fsys FileSystem) FileSystem {
	return &rootFS{fsys}
}

type rootFS struct{ base FileSystem }

func (fsys *rootFS) Open(name string, flags int, mode fs.FileMode) (File, error) {
	f, err := OpenRoot(fsys.base)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return (&rootFile{f}).Open(name, flags, mode)
}

type nopFileCloser struct{ File }

func (nopFileCloser) Close() error { return nil }

type rootFile struct{ File }

func (f *rootFile) Open(name string, flags int, mode fs.FileMode) (File, error) {
	file, err := f.open(name, flags, mode)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: unwrapPathError(err)}
	}
	return &rootFile{file}, nil
}

func (f *rootFile) open(name string, flags int, mode fs.FileMode) (File, error) {
	dir := File(nopFileCloser{f.File})
	defer func() { dir.Close() }()
	setCurrentDirectory := func(cd File) {
		dir.Close()
		dir = cd
	}

	followSymlinkDepth := 0
	followSymlink := func(symlink, target string) error {
		link, err := dir.ReadLink(symlink)
		if err != nil {
			// This error may be EINVAL if the file system was modified
			// concurrently and the directory entry was not pointing to a
			// symbolic link anymore.
			return err
		}

		// Limit the maximum number of symbolic links that would be followed
		// during path resolution; this ensures that if we encounter a loop,
		// we will eventually abort resolving the path.
		if followSymlinkDepth == MaxFollowSymlink {
			return ELOOP
		}
		followSymlinkDepth++

		if target != "" {
			name = link + "/" + target
		} else {
			name = link
		}
		return nil
	}

	depth := filePathDepth(f.Name())
	for {
		if isAbs(name) {
			name = trimLeadingSlash(name)
			d, err := f.openRoot()
			if err != nil {
				return nil, err
			}
			if name == "" {
				return d, nil
			}
			depth = 0
			setCurrentDirectory(d)
		}

		var move int
		var elem string
		elem, name = splitFilePath(name)

		switch elem {
		case ".":
		openFile:
			newFile, err := dir.Open(name, flags|O_NOFOLLOW, mode)
			if err != nil {
				if !errors.Is(err, ELOOP) || ((flags & O_NOFOLLOW) != 0) {
					return nil, err
				}
				switch err := followSymlink(name, ""); err {
				case nil:
					continue
				case EINVAL:
					goto openFile
				default:
					return nil, err
				}
			}
			return newFile, nil

		case "..":
			// This check ensures that we cannot escape the root of the file
			// system when accessing a parent directory.
			if depth == 0 {
				continue
			}
			move = -1
		default:
			move = +1
		}

	openPath:
		d, err := dir.Open(elem, openPathFlags, 0)
		if err != nil {
			if !errors.Is(err, ENOTDIR) {
				return nil, err
			}
			switch err := followSymlink(elem, name); err {
			case nil:
				continue
			case EINVAL:
				goto openPath
			default:
				return nil, err
			}
		}
		depth += move
		setCurrentDirectory(d)
	}
}

func (f *rootFile) openRoot() (File, error) {
	depth := filePathDepth(f.Name())
	if depth == 0 {
		return f.Open(".", O_DIRECTORY, 0)
	}
	dir := File(nopFileCloser{f.File})
	for depth > 0 {
		p, err := dir.Open("..", O_DIRECTORY, 0)
		if err != nil {
			return nil, err
		}
		dir.Close()
		dir = p
	}
	return dir, nil
}

func (f *rootFile) Lstat(name string) (fs.FileInfo, error) {
	return withPath(f, "stat", name, File.Lstat)
}

func (f *rootFile) ReadLink(name string) (string, error) {
	return withPath(f, "readlink", name, File.ReadLink)
}

func withPath[F func(File, string) (R, error), R any](root *rootFile, op, name string, do F) (ret R, err error) {
	dir, base := path.Split(name)
	if dir == "" {
		return do(root.File, base)
	}
	d, err := root.Open(dir, openPathFlags, 0)
	if err != nil {
		return ret, &fs.PathError{Op: op, Path: name, Err: unwrapPathError(err)}
	}
	defer d.Close()
	return do(d.(*rootFile).File, base)
}

func unwrapPathError(err error) error {
	e, ok := err.(*fs.PathError)
	if ok {
		return e.Err
	}
	return err
}

func filePathDepth(path string) (depth int) {
	for {
		path = trimLeadingSlash(path)
		if path == "" {
			return depth
		}
		depth++
		i := strings.IndexByte(path, '/')
		if i < 0 {
			return depth
		}
		path = path[i:]
	}
}

func splitFilePath(path string) (elem, name string) {
	path = trimLeadingSlash(path)
	path = trimTrailingSlash(path)
	i := strings.IndexByte(path, '/')
	if i < 0 {
		return ".", path
	} else {
		return path[:i], trimLeadingSlash(path[i:])
	}
}

func trimLeadingSlash(s string) string {
	i := 0
	for i < len(s) && s[i] == '/' {
		i++
	}
	return s[i:]
}

func trimTrailingSlash(s string) string {
	i := len(s)
	for i > 0 && s[i-1] == '/' {
		i--
	}
	return s[:i]
}

func isAbs(path string) bool {
	return len(path) > 0 && path[0] == '/'
}
