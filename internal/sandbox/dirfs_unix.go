package sandbox

import (
	"io/fs"
	"os"

	"golang.org/x/sys/unix"
)

type dirFS string

func (root dirFS) Open(name string, flags int, mode fs.FileMode) (File, error) {
	dirfd, err := openat(unix.AT_FDCWD, string(root), O_DIRECTORY, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: string(root), Err: err}
	}
	if name = cleanPath(name); name == "/" || name == "." { // root?
		return &dirFile{fd: dirfd, name: "/"}, nil
	}
	defer closeTraceError(dirfd)
	relPath := "/" + trimLeadingSlash(name)
	fd, err := openat(dirfd, name, flags, uint32(mode.Perm()))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: relPath, Err: err}
	}
	return &dirFile{fd: fd, name: relPath}, nil
}

type dirFile struct {
	fd   int
	name string
}

func (f *dirFile) Fd() uintptr {
	return uintptr(f.fd)
}

func (f *dirFile) Name() string {
	return f.name
}

func (f *dirFile) Close() error {
	fd := f.fd
	f.fd = -1
	if fd >= 0 {
		closeTraceError(fd)
	}
	return nil
}

func (f *dirFile) Open(name string, flags int, mode fs.FileMode) (File, error) {
	name = cleanPath(name)
	relPath := f.join(name)
	fd, err := openat(f.fd, name, flags, uint32(mode.Perm()))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: relPath, Err: err}
	}
	return &dirFile{fd: fd, name: relPath}, nil
}

func (f *dirFile) Readv(iovs [][]byte) (int, error) {
	n, err := readv(f.fd, iovs)
	if err != nil {
		err = &fs.PathError{Op: "read", Path: f.name, Err: err}
	}
	return n, err
}

func (f *dirFile) Writev(iovs [][]byte) (int, error) {
	n, err := writev(f.fd, iovs)
	if err != nil {
		err = &fs.PathError{Op: "write", Path: f.name, Err: err}
	}
	return n, err
}

func (f *dirFile) Preadv(iovs [][]byte, offset int64) (int, error) {
	n, err := preadv(f.fd, iovs, offset)
	if err != nil {
		err = &fs.PathError{Op: "pread", Path: f.name, Err: err}
	}
	return n, err
}

func (f *dirFile) Pwritev(iovs [][]byte, offset int64) (int, error) {
	n, err := pwritev(f.fd, iovs, offset)
	if err != nil {
		err = &fs.PathError{Op: "pwrite", Path: f.name, Err: err}
	}
	return n, err
}

func (f *dirFile) Seek(offset int64, whence int) (int64, error) {
	seek, err := lseek(f.fd, offset, whence)
	if err != nil {
		err = &fs.PathError{Op: "seek", Path: f.name, Err: err}
	}
	return seek, err
}

func (f *dirFile) Allocate(offset, length int64) error {
	if err := fallocate(f.fd, offset, length); err != nil {
		return &fs.PathError{Op: "allocate", Path: f.name, Err: err}
	}
	return nil
}

func (f *dirFile) Truncate(size int64) error {
	if err := ftruncate(f.fd, size); err != nil {
		return &fs.PathError{Op: "truncate", Path: f.name, Err: err}
	}
	return nil
}

func (f *dirFile) Sync() error {
	if err := fsync(f.fd); err != nil {
		return &fs.PathError{Op: "sync", Path: f.name, Err: err}
	}
	return nil
}

func (f *dirFile) Datasync() error {
	if err := fdatasync(f.fd); err != nil {
		return &fs.PathError{Op: "datasync", Path: f.name, Err: err}
	}
	return nil
}

func (f *dirFile) Flags() (int, error) {
	flags, err := unix.FcntlInt(uintptr(f.fd), unix.F_GETFL, 0)
	if err != nil {
		return 0, &fs.PathError{Op: "fcntl", Path: f.name, Err: err}
	}
	return flags, nil
}

func (f *dirFile) SetFlags(flags int) error {
	_, err := unix.FcntlInt(uintptr(f.fd), unix.F_SETFL, flags)
	if err != nil {
		return &fs.PathError{Op: "fcntl", Path: f.name, Err: err}
	}
	return nil
}

func (f *dirFile) ReadDirent(buf []byte) (int, error) {
	return ignoreEINTR2(func() (int, error) { return unix.ReadDirent(f.fd, buf) })
}

func (f *dirFile) Stat(name string, flags int) (FileInfo, error) {
	var stat unix.Stat_t
	var err error

	if name == "" {
		err = fstat(f.fd, &stat)
	} else {
		err = fstatat(f.fd, name, &stat, flags)
	}
	if err != nil {
		return FileInfo{}, &fs.PathError{Op: "stat", Path: f.join(name), Err: err}
	}

	info := FileInfo{
		Dev:   uint64(stat.Dev),
		Ino:   uint64(stat.Ino),
		Nlink: uint32(stat.Nlink),
		Mode:  fs.FileMode(stat.Mode & 0777), // perm
		Uid:   uint32(stat.Uid),
		Gid:   uint32(stat.Gid),
		Size:  int64(stat.Size),
		Atime: stat.Atim,
		Mtime: stat.Mtim,
		Ctime: stat.Ctim,
	}

	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFREG:
	case unix.S_IFBLK:
		info.Mode |= fs.ModeDevice
	case unix.S_IFCHR:
		info.Mode |= fs.ModeDevice | fs.ModeCharDevice
	case unix.S_IFDIR:
		info.Mode |= fs.ModeDir
	case unix.S_IFIFO:
		info.Mode |= fs.ModeNamedPipe
	case unix.S_IFLNK:
		info.Mode |= fs.ModeSymlink
	case unix.S_IFSOCK:
		info.Mode |= fs.ModeSocket
	default:
		info.Mode |= fs.ModeIrregular
	}
	return info, nil
}

func (f *dirFile) Readlink(name string, buf []byte) (n int, err error) {
	if name == "" {
		n, err = freadlink(f.fd, buf)
	} else {
		n, err = readlinkat(f.fd, name, buf)
	}
	if err != nil {
		err = &fs.PathError{Op: "readlink", Path: f.join(name), Err: err}
	}
	return n, err
}

func (f *dirFile) Chtimes(name string, times [2]Timespec, flags int) error {
	var err error
	if name == "" {
		err = futimens(f.fd, &times)
	} else {
		err = utimensat(f.fd, name, &times, flags)
	}
	if err != nil {
		return &fs.PathError{Op: "chtimes", Path: f.join(name), Err: err}
	}
	return err
}

func (f *dirFile) Mkdir(name string, mode fs.FileMode) error {
	if err := mkdirat(f.fd, name, uint32(mode.Perm())); err != nil {
		return &fs.PathError{Op: "mkdir", Path: f.join(name), Err: err}
	}
	return nil
}

func (f *dirFile) Rmdir(name string) error {
	if err := unlinkat(f.fd, name, unix.AT_REMOVEDIR); err != nil {
		return &fs.PathError{Op: "rmdir", Path: f.join(name), Err: err}
	}
	return nil
}

func (f *dirFile) Rename(oldName string, newDir File, newName string) error {
	fd1 := f.fd
	fd2 := int(newDir.Fd())
	if err := renameat(fd1, oldName, fd2, newName); err != nil {
		path1 := f.join(oldName)
		path2 := joinPath(newDir.Name(), newName)
		return &os.LinkError{Op: "rename", Old: path1, New: path2, Err: err}
	}
	return nil
}

func (f *dirFile) Link(oldName string, newDir File, newName string, flags int) error {
	linkFlags := 0
	if (flags & AT_SYMLINK_NOFOLLOW) == 0 {
		linkFlags |= unix.AT_SYMLINK_FOLLOW
	}
	fd1 := f.fd
	fd2 := int(newDir.Fd())
	if err := linkat(fd1, oldName, fd2, newName, linkFlags); err != nil {
		path1 := f.join(oldName)
		path2 := joinPath(newDir.Name(), newName)
		return &os.LinkError{Op: "link", Old: path1, New: path2, Err: err}
	}
	return nil
}

func (f *dirFile) Symlink(oldName string, newName string) error {
	if err := symlinkat(oldName, f.fd, newName); err != nil {
		return &fs.PathError{Op: "symlink", Path: f.join(newName), Err: err}
	}
	return nil
}

func (f *dirFile) Unlink(name string) error {
	if err := unlinkat(f.fd, name, 0); err != nil {
		return &fs.PathError{Op: "unlink", Path: f.join(name), Err: err}
	}
	return nil
}

func (f *dirFile) join(name string) string {
	return joinPath(f.name, name)
}
