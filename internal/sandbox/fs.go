package sandbox

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os/user"
	"path"
	"strconv"
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
		link, err := readlink(dir, path)
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

func Lstat(fsys FileSystem, name string) (FileInfo, error) {
	return withRoot2(fsys, func(dir File) (FileInfo, error) { return dir.Stat(name, AT_SYMLINK_NOFOLLOW) })
}

func Stat(fsys FileSystem, name string) (FileInfo, error) {
	return withRoot2(fsys, func(dir File) (FileInfo, error) { return dir.Stat(name, 0) })
}

func ReadFile(fsys FileSystem, name string, flags int) ([]byte, error) {
	f, err := fsys.Open(name, flags|O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	s, err := f.Stat("", 0)
	if err != nil {
		return nil, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, err
	}
	b := make([]byte, s.Size)
	v := make([][]byte, 1)
	n := 0
	for n < len(b) {
		v[0] = b[n:]
		rn, err := f.Readv(v)
		if rn > 0 {
			n += rn
		}
		if err != nil || rn == 0 {
			return b[:n], err
		}
	}
	return b, nil
}

func WriteFile(fsys FileSystem, name string, data []byte, mode fs.FileMode) error {
	f, err := fsys.Open(name, O_CREAT|O_WRONLY|O_TRUNC|O_EXCL, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Writev([][]byte{data})
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
	return withRoot1(fsys, func(dir File) error { return dir.Link(oldName, dir, newName, AT_SYMLINK_NOFOLLOW) })
}

func Symlink(fsys FileSystem, oldName, newName string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Symlink(oldName, newName) })
}

func Readlink(fsys FileSystem, name string) (string, error) {
	return withRoot2(fsys, func(dir File) (string, error) { return readlink(dir, name) })
}

func readlink(dir File, name string) (string, error) {
	b := make([]byte, 256)
	for {
		n, err := dir.Readlink(name, b)
		if err != nil {
			return "", err
		}
		if n < len(b) {
			return string(b[:n]), nil
		}
		if len(b) > _PATH_MAX {
			return "", &fs.PathError{Op: "readlink", Path: name, Err: ENAMETOOLONG}
		}
		b = make([]byte, 2*len(b))
	}
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

// File is an interface representing files opened from a file system.
type File interface {
	// Returns the file descriptor number for the underlying kernel handle for
	// the file.
	Fd() uintptr

	// Returns the canonical name of the file on the file system.
	//
	// Assuming the file system is not modified concurrently, a file opened at
	// the location returned by this method will point to the same resource.
	Name() string

	// Closes the file.
	//
	// This method must be opened when the program does not need the file
	// anymore. Attempting to use the file after it was closed will cause
	// the methods to return errors.
	Close() error

	// Opens a file at the given name, relative to the file's position in the
	// file system.
	//
	// The file must point to a directory or the method errors with ENOTDIR.
	Open(name string, flags int, mode fs.FileMode) (File, error)

	Readv(iovs [][]byte) (int, error)

	Writev(iovs [][]byte) (int, error)

	Preadv(iovs [][]byte, offset int64) (int, error)

	Pwritev(iovs [][]byte, offset int64) (int, error)

	Seek(offset int64, whence int) (int64, error)

	Allocate(offset, length int64) error

	Truncate(size int64) error

	Sync() error

	Datasync() error

	Flags() (int, error)

	SetFlags(flags int) error

	ReadDirent(buf []byte) (int, error)

	Stat(name string, flags int) (FileInfo, error)

	Readlink(name string, buf []byte) (int, error)

	Chtimes(name string, times [2]Timespec, flags int) error

	Mkdir(name string, mode fs.FileMode) error

	Rmdir(name string) error

	Rename(oldName string, newDir File, newName string) error

	Link(oldName string, newDir File, newName string, flags int) error

	Symlink(oldName, newName string) error

	Unlink(name string) error
}

// FileInfo is a type similar to fs.FileInfo or syscall.Stat_t on unix systems.
// It contains metadata about an entry on the file system.
type FileInfo struct {
	Dev   uint64
	Ino   uint64
	Nlink uint64
	Mode  fs.FileMode
	Gid   uint32
	Uid   uint32
	Size  int64
	Atime Timespec
	Mtime Timespec
	Ctime Timespec
}

func (info FileInfo) String() string {
	group, name := "none", "nobody"

	g, err := user.LookupGroupId(strconv.Itoa(int(info.Gid)))
	if err == nil {
		group = g.Name
	}

	u, err := user.LookupId(strconv.Itoa(int(info.Uid)))
	if err == nil {
		name = u.Username
	}

	return fmt.Sprintf("%s %d %s %s %d %s",
		info.Mode,
		info.Nlink,
		name,
		group,
		info.Size,
		time.Unix(info.Mtime.Unix()).Format(time.Stamp),
	)
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
	return &fsFile{fsys: fsys.base, File: f}, nil
}

func (fsys *fsFileSystem) Stat(name string) (fs.FileInfo, error) {
	if !fs.ValidPath(name) {
		return nil, fsError("stat", name, fs.ErrNotExist)
	}
	s, err := Stat(fsys.base, name)
	if err != nil {
		return nil, fsError("stat", name, err)
	}
	return &fsFileInfo{name: path.Base(name), stat: s}, nil
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
	dir  *dirbuf
	File
}

func (f *fsFile) Stat() (fs.FileInfo, error) {
	stat, err := f.File.Stat("", AT_SYMLINK_NOFOLLOW)
	if err != nil {
		return nil, err
	}
	name := path.Base(f.File.Name())
	return &fsFileInfo{name: name, stat: stat}, nil
}

func (f *fsFile) Read(b []byte) (int, error) {
	iovs := [][]byte{b}
	read := 0
	for {
		n, err := f.File.Readv(iovs)
		if n > 0 {
			read += n
		}
		if read == len(b) {
			return read, nil
		}
		if err != nil {
			return read, err
		}
		if n == 0 {
			return read, io.EOF
		}
		iovs[0] = b[read:]
	}
}

func (f *fsFile) ReadAt(b []byte, off int64) (int, error) {
	iovs := [][]byte{b}
	read := 0
	for {
		n, err := f.File.Preadv(iovs, off)
		if n > 0 {
			off += int64(n)
			read += n
		}
		if read == len(b) {
			return read, nil
		}
		if err != nil {
			return read, err
		}
		if n == 0 {
			return read, io.EOF
		}
		iovs[0] = b[read:]
	}
}

func (f *fsFile) ReadDir(n int) ([]fs.DirEntry, error) {
	var dirents []fs.DirEntry

	if n > 0 {
		dirents = make([]fs.DirEntry, 0, n)
	}
	if f.dir == nil {
		f.dir = &dirbuf{file: f.File}
	}

	for {
		name, mode, err := f.dir.readDirEntry()
		if err != nil {
			if err == io.EOF && n <= 0 {
				err = nil
			}
			f.dir = nil
			return dirents, err
		}
		dirents = append(dirents, &fsDirEntry{
			file: f,
			name: name,
			mode: mode,
		})
		if n == len(dirents) {
			return dirents, nil
		}
	}
}

var (
	_ io.ReaderAt = (*fsFile)(nil)
)

type fsFileInfo struct {
	name string
	stat FileInfo
}

func (info *fsFileInfo) IsDir() bool {
	return info.stat.Mode.IsDir()
}

func (info *fsFileInfo) Mode() fs.FileMode {
	return info.stat.Mode
}

func (info *fsFileInfo) ModTime() time.Time {
	return time.Unix(info.stat.Mtime.Unix())
}

func (info *fsFileInfo) Name() string {
	return info.name
}

func (info *fsFileInfo) Size() int64 {
	return info.stat.Size
}

func (info *fsFileInfo) Sys() any {
	return &info.stat
}

func (info *fsFileInfo) String() string {
	return info.stat.String() + " " + info.name
}

type fsDirEntry struct {
	file *fsFile
	name string
	mode fs.FileMode
}

func (dirent *fsDirEntry) IsDir() bool {
	return dirent.mode.IsDir()
}

func (dirent *fsDirEntry) Type() fs.FileMode {
	return dirent.mode
}

func (dirent *fsDirEntry) Name() string {
	return dirent.name
}

func (dirent *fsDirEntry) Info() (fs.FileInfo, error) {
	name := dirent.Name()
	stat, err := dirent.file.File.Stat(name, AT_SYMLINK_NOFOLLOW)
	if err != nil {
		return nil, fsError("stat", dirent.Name(), err)
	}
	return &fsFileInfo{name: name, stat: stat}, nil
}
