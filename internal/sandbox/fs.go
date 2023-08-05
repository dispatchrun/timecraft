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
	MaxFollowSymlink = 10
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

// Create creates and opens a file on a file system. The name is the location
// where the file is created and the mode is used to set permissions.
func Create(fsys FileSystem, name string, mode fs.FileMode) (File, error) {
	return fsys.Open(name, O_CREAT|O_TRUNC|O_WRONLY, mode)
}

// Open opens a file with the given name on a file system.
func Open(fsys FileSystem, name string) (File, error) {
	return fsys.Open(name, O_RDONLY, 0)
}

// OpenDir opens a directory with the given name on the file system.
func OpenDir(fsys FileSystem, name string) (File, error) {
	return fsys.Open(name, O_DIRECTORY, 0)
}

// OpenRoot opens the root directory of a file system.
func OpenRoot(fsys FileSystem) (File, error) {
	return OpenDir(fsys, "/")
}

// Lstat returns information about a file on a file system.
//
// Is the name points to a location where a symbolic link exists, the function
// returns information about the link itself.
func Lstat(fsys FileSystem, name string) (FileInfo, error) {
	return withRoot2(fsys, func(dir File) (FileInfo, error) { return dir.Stat(name, AT_SYMLINK_NOFOLLOW) })
}

// Stat returns information about a file on a file system.
//
// Is the name points to a location where a symbolic link exists, the function
// returns information about the link target.
func Stat(fsys FileSystem, name string) (FileInfo, error) {
	return withRoot2(fsys, func(dir File) (FileInfo, error) { return dir.Stat(name, 0) })
}

// ReadFile reads the content of a file on a file system. The name represents
// the location where the file is recorded on the file system. The flags are
// passed to configure how the file is opened (e.g. passing O_NOFOLLOW will
// fail if a symbolic link exists at that location).
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

// WriteFile writes a file on a file system.
func WriteFile(fsys FileSystem, name string, data []byte, mode fs.FileMode) error {
	f, err := fsys.Open(name, O_CREAT|O_WRONLY|O_TRUNC|O_EXCL, mode)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Writev([][]byte{data})
	return err
}

// MkdirAll creates all directories to form the given path name on a file
// system. The mode is used to set the permissions of each new directory,
// permissions of existing directories are left untouched.
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

// Mkdir creates a directory on a file system. The mode is used to set the
// permissions of the new directory.
func Mkdir(fsys FileSystem, name string, mode fs.FileMode) error {
	return withRoot1(fsys, func(dir File) error { return dir.Mkdir(name, mode) })
}

// Rmdir removes an empty directory from a file system.
func Rmdir(fsys FileSystem, name string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Rmdir(name) })
}

// Link creates a hard link between the old and new names passed as arguments.
func Link(fsys FileSystem, oldName, newName string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Link(oldName, dir, newName, AT_SYMLINK_NOFOLLOW) })
}

// Symlink creates a symbolic link to a file system location.
func Symlink(fsys FileSystem, oldName, newName string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Symlink(oldName, newName) })
}

// Readlink reads the target of a symbolic link located at the given path name
// on a file system.
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
		if len(b) > PATH_MAX {
			return "", &fs.PathError{Op: "readlink", Path: name, Err: ENAMETOOLONG}
		}
		b = make([]byte, 2*len(b))
	}
}

// Unlink removes a file or symbolic link from a file system.
func Unlink(fsys FileSystem, name string) error {
	return withRoot1(fsys, func(dir File) error { return dir.Unlink(name) })
}

// Rename changes the name referencing a file, symbolic link, or directory on a
// file system.
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

	// Readv reads data from the current seek offset of the file into the list
	// of vectors passed as arguments.
	//
	// The method returns the number of bytes read, which may be less than the
	// total size of the read buffers, even in the absence of errors.
	//
	// When the end of file is reached, the method returns zero and a nil error
	// (it does not return io.EOF).
	Readv(iovs [][]byte) (int, error)

	// Writev writes data from the list of vectors to the current seek offset
	// of the file.
	//
	// The method returns the number of bytes written, which may be less than
	// the total size of the write buffers, even in the absence of errors.
	Writev(iovs [][]byte) (int, error)

	// Preadv reads data from the given seek offset into the list of vectors
	// passed as arguments.
	//
	// The method returns the number of bytes read, which may be less than the
	// total size of the read buffers, even in the absence of errors.
	//
	// When the end of file is reached, the method returns zero and a nil error
	// (it does not return io.EOF).
	Preadv(iovs [][]byte, offset int64) (int, error)

	// Pwritev writes data from the list of vectors at the given seek offset.
	//
	// The method returns the number of bytes written, which may be less than
	// the total size of the write buffers, even in the absence of errors.
	Pwritev(iovs [][]byte, offset int64) (int, error)

	// Seek positions the seek offset of the file at the given location, which
	// is interpreted relative to the whence value. The whence may be SEEK_SET,
	// SEEK_CUR, or SEEK_END to describe how to compute the final seek offset.
	Seek(offset int64, whence int) (int64, error)

	// Pre-allocates storage for the file at the given offset and length. If the
	// sum of offset and length exceeds the current size, the file is extended
	// as if Truncate(offset + length) had been called.
	Allocate(offset, length int64) error

	// Sets the file to the given size.
	//
	// If the size is shorter than the current file size, its content is
	// truncated and the data at the end of the file is dropped.
	//
	// If the size is larger than the current file size, zero bytes are appended
	// at the end to match the requested size.
	Truncate(size int64) error

	// Blocks until all buffered changes have been flushed to the underyling
	// storage device.
	//
	// Syncing includes writing metdata such as mutations to a directory.
	Sync() error

	// Datasync is similar to Sync but it only synchronizes writes to a file
	// content.
	Datasync() error

	// Returns the bitset of flags currently set on the file, which is a
	// combination of O_* flags such as those that can be passed to Open.
	//
	// The set of flags supported by the file depends on the underlying type.
	Flags() (int, error)

	// Changes the bitset of flags set on the file. The flags are a combination
	// of O_* flags such as those that can be passed to Open.
	//
	// The set of flags supported by the file depends on the underlying type.
	SetFlags(flags int) error

	// Read directory entries into the given buffer. The caller must be aware of
	// the way directory entries are laid out by the underlying file system to
	// interpret the content.
	//
	// The method returns the number of bytes written to buf.
	ReadDirent(buf []byte) (int, error)

	// Looks up and return file metdata.
	//
	// If the receiver is a directory, a name may be given to represent the file
	// to retrieve metdata for, relative to the directory. The flags may be
	// AT_SYMLINK_NOFOLLOW to retrieve metdata for a symbolic link instead of
	// its target.
	//
	// If the name is empty, flags are ignored and the method returns metdata
	// for the receiver.
	Stat(name string, flags int) (FileInfo, error)

	// Reads the target of a symbolic link into buf.
	//
	// If the name is empty, the method assumes that the receiver is a file
	// opened on a symbolic link and returns the receiver's target.
	//
	// The method returns the number of bytes written to buf.
	Readlink(name string, buf []byte) (int, error)

	// Changes the access and modification time of a file.
	//
	// The access time is the first Timespec value, the modification time is the
	// second. Either of the Timespec values may have their nanosecond field set
	// to UTIME_OMIT to ignore it, or UTIME_NOW to set it to the current time.
	//
	// If the receiver is a directory, a name may be given to represent the file
	// to set the times for, relative to the directory. The flags may be
	// AT_SYMLINK_NOFOLLOW to change the times of a symbolic link instead of its
	// target (note that not all file systems may support it).
	//
	// If the name is empty, flags are ignored and the method changes times of
	// the receiver.
	Chtimes(name string, times [2]Timespec, flags int) error

	// Creates a directory at the named location.
	//
	// The method assumes that the receiver is a directory and resolves the path
	// relative to it.
	//
	// The mode sets permissions on the newly created directory.
	Mkdir(name string, mode fs.FileMode) error

	// Removes an empty directory at a named location.
	//
	// The method assumes that the receiver is a directory and resolves the path
	// relative to it.
	Rmdir(name string) error

	// Moves a file to a new location.
	//
	// The old name is the path to the file to be moved, relative to the
	// receiver, which is expected to refer to a directory.
	//
	// The new name is interpreted relative to the directory passed as argument,
	// which may or may not be the same as the receiver, but must be on the same
	// file system.
	Rename(oldName string, newDir File, newName string) error

	// Creates a hard link to a named location.
	//
	// The old name is the path to the file to be linked, relative to the
	// receiver, which is expected to refer to a directory.
	//
	// The new name is interpreted relative to the directory passed as argument,
	// which may or may not be the same as the reciver, but must be on the same
	// file system.
	//
	// The flags may be AT_SYMLINK_NOFOLLOW to create a link to a symbolic link
	// instead of its target.
	Link(oldName string, newDir File, newName string, flags int) error

	// Creates a symbolic link to a named location.
	//
	// The old name may be an absolute or relative location, and does not need
	// to exist on the file system.
	//
	// The new name is interpreted relative to the receiver, which is expected
	// to refer to a directory.
	Symlink(oldName, newName string) error

	// Removes a file or symbolic link from the file system.
	//
	// The method is not idempotent, an error is returned if no files exist at
	// the location.
	//
	// Unlinking a file only drops the name referencing it, its content is only
	// reclaimed by the file system once all open references have been closed.
	Unlink(name string) error
}

// FileInfo is a type similar to fs.FileInfo or syscall.Stat_t on unix systems.
// It contains metadata about an entry on the file system.
type FileInfo struct {
	Dev   uint64
	Ino   uint64
	Nlink uint64
	Mode  fs.FileMode
	Uid   uint32
	Gid   uint32
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
// This method is useful to run the standard testing/fstest test suite against
// instances of the FileSystem interface.
//
// The returned fs.FS implements fs.StatFS.
func FS(fsys FileSystem) fs.FS { return &fsFileSystem{fsys} }

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
		if read == len(b) || err != nil {
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
		if read == len(b) || err != nil {
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
