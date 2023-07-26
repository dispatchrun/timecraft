package sandbox

import (
	"io/fs"
	"os"
	"path"
	"time"

	"golang.org/x/sys/unix"
)

type dirFS string

func (root dirFS) Open(name string, flags int, mode fs.FileMode) (File, error) {
	relPath := path.Join("/", name)
	absPath := path.Join(string(root), relPath)
	fd, err := openat(unix.AT_FDCWD, absPath, flags, uint32(mode.Perm()))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: relPath, Err: err}
	}
	return dirFile{os.NewFile(uintptr(fd), relPath)}, nil
}

type dirFile struct{ *os.File }

func (f dirFile) Open(name string, flags int, mode fs.FileMode) (File, error) {
	relPath := f.join(name)
	fd, err := openat(f.fd(), name, flags, uint32(mode.Perm()))
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: relPath, Err: err}
	}
	return dirFile{os.NewFile(uintptr(fd), relPath)}, nil
}

func (f dirFile) Stat() (fs.FileInfo, error) {
	info := &dirFileInfo{name: f.Name()}
	err := fstat(f.fd(), &info.stat)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: info.name, Err: err}
	}
	return info, nil
}

func (f dirFile) Datasync() error {
	if err := fdatasync(f.fd()); err != nil {
		return &fs.PathError{Op: "datasync", Path: f.Name(), Err: err}
	}
	return nil
}

func (f dirFile) Lstat(name string) (fs.FileInfo, error) {
	info := &dirFileInfo{name: f.join(name)}
	err := fstatat(f.fd(), name, &info.stat, unix.AT_SYMLINK_NOFOLLOW)
	if err != nil {
		return nil, &fs.PathError{Op: "stat", Path: info.name, Err: err}
	}
	return info, nil
}

func (f dirFile) ReadLink(name string) (string, error) {
	s, err := readlinkat(f.fd(), name)
	if err != nil {
		return "", &fs.PathError{Op: "readlink", Path: f.join(name), Err: err}
	}
	return s, nil
}

func (f dirFile) SetFlags(flags int) error {
	_, err := unix.FcntlInt(f.Fd(), unix.F_SETFL, flags)
	if err != nil {
		return &fs.PathError{Op: "fcntl", Path: f.Name(), Err: err}
	}
	return nil
}

func (f dirFile) fd() int {
	return int(f.Fd())
}

func (f dirFile) join(name string) string {
	return path.Join(f.Name(), name)
}

type dirFileInfo struct {
	stat unix.Stat_t
	name string
}

func (info *dirFileInfo) Name() string {
	return path.Base(info.name)
}

func (info *dirFileInfo) Size() int64 {
	return info.stat.Size
}

func (info *dirFileInfo) Mode() (mode fs.FileMode) {
	mode = fs.FileMode(info.stat.Mode & 0777) // perm
	switch info.stat.Mode & unix.S_IFMT {
	case unix.S_IFREG:
	case unix.S_IFBLK:
		mode |= fs.ModeDevice
	case unix.S_IFCHR:
		mode |= fs.ModeDevice | fs.ModeCharDevice
	case unix.S_IFDIR:
		mode |= fs.ModeDir
	case unix.S_IFIFO:
		mode |= fs.ModeNamedPipe
	case unix.S_IFLNK:
		mode |= fs.ModeSymlink
	case unix.S_IFSOCK:
		mode |= fs.ModeSocket
	default:
		mode |= fs.ModeIrregular
	}
	return mode
}

func (info *dirFileInfo) ModTime() time.Time {
	return time.Unix(info.stat.Mtim.Unix())
}

func (info *dirFileInfo) IsDir() bool {
	return (info.stat.Mode & unix.S_IFDIR) != 0
}

func (info *dirFileInfo) Sys() any {
	return &info.stat
}
