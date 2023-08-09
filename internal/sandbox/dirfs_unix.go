package sandbox

import (
	"fmt"
	"io/fs"
	"sync/atomic"

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
		return nil, err
	}
	return newFile(nil, dirfd), nil
}

func newFile(dir *dirFile, fd int) *dirFile {
	f := &dirFile{dir: dir, fd: fd}
	if dir != nil {
		dir.ref()
	}
	f.ref()
	return f
}

type dirFile struct {
	dir  *dirFile
	refc atomic.Int32
	once atomic.Int32
	fd   int
}

func (f *dirFile) ref() {
	f.refc.Add(1)
}

func (f *dirFile) unref() {
	if f.refc.Add(-1) == 0 {
		closeTraceError(f.fd)
		f.fd = -1
		if f.dir != nil {
			f.dir.unref()
		}
	}
}

func (f *dirFile) String() string {
	return fmt.Sprintf("&sandbox.dirFile{fd:%d}", f.fd)
}

func (f *dirFile) Fd() uintptr {
	return uintptr(f.fd)
}

func (f *dirFile) Close() error {
	if f.once.Swap(1) == 0 {
		f.unref()
	}
	return nil
}

func (f *dirFile) openSelf() (File, error) {
	fd, err := dup(f.fd)
	if err != nil {
		return nil, err
	}
	return newFile(f.dir, fd), nil
}

func (f *dirFile) openParent() (File, error) {
	if f.dir != nil { // not already at the root?
		f = f.dir
	}
	return f.openSelf()
}

func (f *dirFile) openRoot() (File, error) {
	for f.dir != nil { // walk up to root
		f = f.dir
	}
	return f.openSelf()
}

func (f *dirFile) openFile(name string, flags int, mode fs.FileMode) (File, error) {
	fd, err := openat(f.fd, name, flags|O_NOFOLLOW, uint32(mode.Perm()))
	if err != nil {
		return nil, err
	}
	return newFile(f, fd), nil
}

func (f *dirFile) open(name string, flags int, mode fs.FileMode) (File, error) {
	switch name {
	case ".":
		return f.openSelf()
	case "..":
		return f.openParent()
	default:
		return f.openFile(name, flags, mode)
	}
}

func (f *dirFile) Open(name string, flags int, mode fs.FileMode) (File, error) {
	if fspath.IsRoot(name) {
		return f.openRoot()
	}
	return ResolvePath(f, name, flags, func(d *dirFile, name string) (File, error) {
		return d.open(name, flags, mode)
	})
}

func (f *dirFile) Readv(iovs [][]byte) (int, error) {
	return readv(f.fd, iovs)
}

func (f *dirFile) Writev(iovs [][]byte) (int, error) {
	return writev(f.fd, iovs)
}

func (f *dirFile) Preadv(iovs [][]byte, offset int64) (int, error) {
	return preadv(f.fd, iovs, offset)
}

func (f *dirFile) Pwritev(iovs [][]byte, offset int64) (int, error) {
	return pwritev(f.fd, iovs, offset)
}

func (f *dirFile) Seek(offset int64, whence int) (int64, error) {
	return lseek(f.fd, offset, whence)
}

func (f *dirFile) Allocate(offset, length int64) error {
	return fallocate(f.fd, offset, length)
}

func (f *dirFile) Truncate(size int64) error {
	return ftruncate(f.fd, size)
}

func (f *dirFile) Sync() error {
	return fsync(f.fd)
}

func (f *dirFile) Datasync() error {
	return fdatasync(f.fd)
}

func (f *dirFile) Flags() (int, error) {
	return unix.FcntlInt(uintptr(f.fd), unix.F_GETFL, 0)
}

func (f *dirFile) SetFlags(flags int) error {
	_, err := unix.FcntlInt(uintptr(f.fd), unix.F_SETFL, flags)
	return err
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
			return FileInfo{}, err
		}
		if (stat.Mode & unix.S_IFMT) == unix.S_IFLNK {
			if (flags & AT_SYMLINK_NOFOLLOW) == 0 {
				return FileInfo{}, ELOOP
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
	return ResolvePath(f, name, O_NOFOLLOW, func(d *dirFile, name string) (int, error) {
		if name == "" {
			return freadlink(d.fd, buf)
		} else {
			return readlinkat(d.fd, name, buf)
		}
	})
}

func (f *dirFile) Chtimes(name string, times [2]Timespec, flags int) error {
	return resolvePath1(f, name, openFlags(flags), func(d *dirFile, name string) error {
		if name == "" {
			return futimens(d.fd, &times)
		} else {
			return utimensat(d.fd, name, &times, AT_SYMLINK_NOFOLLOW)
		}
	})
}

func (f *dirFile) Mkdir(name string, mode fs.FileMode) error {
	return resolvePath1(f, name, O_NOFOLLOW, func(d *dirFile, name string) error {
		return mkdirat(d.fd, name, uint32(mode.Perm()))
	})
}

func (f *dirFile) Rmdir(name string) error {
	return resolvePath1(f, name, O_NOFOLLOW, func(d *dirFile, name string) error {
		return unlinkat(d.fd, name, unix.AT_REMOVEDIR)
	})
}

func (f1 *dirFile) Rename(oldName string, newDir File, newName string) error {
	f2, ok := newDir.(*dirFile)
	if !ok {
		return EXDEV
	}
	return resolvePath1(f1, oldName, O_NOFOLLOW, func(d1 *dirFile, name1 string) error {
		return resolvePath1(f2, newName, O_NOFOLLOW, func(d2 *dirFile, name2 string) error {
			return renameat(d1.fd, name1, d2.fd, name2)
		})
	})
}

func (f1 *dirFile) Link(oldName string, newDir File, newName string, flags int) error {
	f2, ok := newDir.(*dirFile)
	if !ok {
		return EXDEV
	}
	oflags := openFlags(flags)
	return resolvePath1(f1, oldName, oflags, func(d1 *dirFile, name1 string) error {
		return resolvePath1(f2, newName, oflags, func(d2 *dirFile, name2 string) error {
			return linkat(d1.fd, name1, d2.fd, name2, 0)
		})
	})
}

func (f *dirFile) Symlink(oldName string, newName string) error {
	return resolvePath1(f, newName, O_NOFOLLOW, func(d *dirFile, name string) error {
		return symlinkat(oldName, d.fd, name)
	})
}

func (f *dirFile) Unlink(name string) error {
	return resolvePath1(f, name, O_NOFOLLOW, func(d *dirFile, name string) error {
		return unlinkat(d.fd, name, 0)
	})
}

func openFlags(flags int) int {
	if (flags & AT_SYMLINK_NOFOLLOW) != 0 {
		return O_NOFOLLOW
	}
	return 0
}

func resolvePath1(d *dirFile, name string, flags int, do func(*dirFile, string) error) error {
	_, err := ResolvePath(d, name, flags, func(d *dirFile, name string) (_ struct{}, err error) {
		err = do(d, name)
		return
	})
	return err
}
