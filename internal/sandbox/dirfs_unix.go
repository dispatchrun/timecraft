package sandbox

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/stealthrocket/timecraft/internal/sandbox/fspath"
	"golang.org/x/sys/unix"
)

type dirFS struct {
	root string
}

func (fsys *dirFS) Open(name string, flags int, mode fs.FileMode) (File, error) {
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

func (fsys *dirFS) openRoot() (File, error) {
	dirfd, err := openat(unix.AT_FDCWD, fsys.root, O_DIRECTORY, 0)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: "/", Err: err}
	}
	return &dirFile{fsys: fsys, fd: dirfd, name: "/"}, nil
}

type dirFile struct {
	fsys *dirFS
	fd   int
	name string
}

func (f *dirFile) String() string {
	return fmt.Sprintf("&sandbox.dirFile{fd:%d,name:%q}", f.fd, f.name)
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
	if fspath.IsRoot(name) {
		return f.fsys.openRoot()
	}
	relPath := f.join(name)
	return ResolvePath(f, name, flags, func(d *dirFile, name string) (File, error) {
		fd, err := openat(d.fd, name, flags|O_NOFOLLOW, uint32(mode.Perm()))
		if err != nil {
			return nil, &fs.PathError{Op: "open", Path: relPath, Err: err}
		}
		return &dirFile{fsys: f.fsys, fd: fd, name: relPath}, nil
	})
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
	return ResolvePath(f, name, openFlags(flags), func(d *dirFile, name string) (FileInfo, error) {
		var stat unix.Stat_t
		var err error

		if name == "" {
			err = fstat(d.fd, &stat)
		} else {
			err = fstatat(d.fd, name, &stat, AT_SYMLINK_NOFOLLOW)
		}
		if err != nil {
			return FileInfo{}, &fs.PathError{Op: "stat", Path: d.join(name), Err: err}
		}
		if (stat.Mode & unix.S_IFMT) == unix.S_IFLNK {
			if (flags & AT_SYMLINK_NOFOLLOW) == 0 {
				return FileInfo{}, &fs.PathError{Op: "stat", Path: d.join(name), Err: ELOOP}
			}
		}

		info := FileInfo{
			Dev:   uint64(stat.Dev),
			Ino:   uint64(stat.Ino),
			Nlink: uint64(stat.Nlink),
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
	})
}

func (f *dirFile) Readlink(name string, buf []byte) (int, error) {
	return ResolvePath(f, name, O_NOFOLLOW, func(d *dirFile, name string) (n int, err error) {
		if name == "" {
			n, err = freadlink(d.fd, buf)
		} else {
			n, err = readlinkat(d.fd, name, buf)
		}
		if err != nil {
			err = &fs.PathError{Op: "readlink", Path: d.join(name), Err: err}
		}
		return n, err
	})
}

func (f *dirFile) Chtimes(name string, times [2]Timespec, flags int) error {
	return resolvePath1(f, name, openFlags(flags), func(d *dirFile, name string) (err error) {
		if name == "" {
			err = futimens(d.fd, &times)
		} else {
			err = utimensat(d.fd, name, &times, AT_SYMLINK_NOFOLLOW)
		}
		if err != nil {
			return &fs.PathError{Op: "chtimes", Path: f.join(name), Err: err}
		}
		return err
	})
}

func (f *dirFile) Mkdir(name string, mode fs.FileMode) error {
	return resolvePath1(f, name, O_NOFOLLOW, func(d *dirFile, name string) error {
		if err := mkdirat(d.fd, name, uint32(mode.Perm())); err != nil {
			return &fs.PathError{Op: "mkdir", Path: f.join(name), Err: err}
		}
		return nil
	})
}

func (f *dirFile) Rmdir(name string) error {
	return resolvePath1(f, name, O_NOFOLLOW, func(d *dirFile, name string) error {
		if err := unlinkat(d.fd, name, unix.AT_REMOVEDIR); err != nil {
			return &fs.PathError{Op: "rmdir", Path: f.join(name), Err: err}
		}
		return nil
	})
}

func (f1 *dirFile) Rename(oldName string, newDir File, newName string) error {
	f2, ok := newDir.(*dirFile)
	if !ok {
		path1 := f1.join(oldName)
		path2 := f2.join(newName)
		return &os.LinkError{Op: "rename", Old: path1, New: path2, Err: EXDEV}
	}
	return resolvePath1(f1, oldName, O_NOFOLLOW, func(d1 *dirFile, name1 string) error {
		return resolvePath1(f2, newName, O_NOFOLLOW, func(d2 *dirFile, name2 string) error {
			if err := renameat(d1.fd, name1, d2.fd, name2); err != nil {
				path1 := d1.join(name1)
				path2 := d2.join(name2)
				return &os.LinkError{Op: "rename", Old: path1, New: path2, Err: err}
			}
			return nil
		})
	})
}

func (f1 *dirFile) Link(oldName string, newDir File, newName string, flags int) error {
	f2, ok := newDir.(*dirFile)
	if !ok {
		path1 := f1.join(oldName)
		path2 := f2.join(newName)
		return &os.LinkError{Op: "rename", Old: path1, New: path2, Err: EXDEV}
	}
	oflags := openFlags(flags)
	return resolvePath1(f1, oldName, oflags, func(d1 *dirFile, name1 string) error {
		return resolvePath1(f2, newName, oflags, func(d2 *dirFile, name2 string) error {
			if err := linkat(d1.fd, name1, d2.fd, name2, 0); err != nil {
				path1 := d1.join(name1)
				path2 := f2.join(name2)
				return &os.LinkError{Op: "link", Old: path1, New: path2, Err: err}
			}
			return nil
		})
	})
}

func (f *dirFile) Symlink(oldName string, newName string) error {
	return resolvePath1(f, newName, O_NOFOLLOW, func(d *dirFile, name string) error {
		if err := symlinkat(oldName, d.fd, name); err != nil {
			return &fs.PathError{Op: "symlink", Path: f.join(newName), Err: err}
		}
		return nil
	})
}

func (f *dirFile) Unlink(name string) error {
	return resolvePath1(f, name, O_NOFOLLOW, func(d *dirFile, name string) error {
		if err := unlinkat(d.fd, name, 0); err != nil {
			return &fs.PathError{Op: "unlink", Path: f.join(name), Err: err}
		}
		return nil
	})
}

func (f *dirFile) join(name string) string {
	return fspath.Join(f.name, name)
}

func openFlags(flags int) int {
	if (flags & AT_SYMLINK_NOFOLLOW) != 0 {
		return O_NOFOLLOW
	}
	return 0
}

func resolvePath1[F File](d F, name string, flags int, do func(F, string) error) error {
	_, err := ResolvePath(d, name, flags, func(d F, name string) (_ struct{}, err error) {
		err = do(d, name)
		return
	})
	return err
}
