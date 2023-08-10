package sandbox

import (
	"fmt"
	"io/fs"
	"sync"

	"github.com/stealthrocket/timecraft/internal/sandbox/fspath"
	"golang.org/x/sys/unix"
)

type dirFS struct {
	root string
}

func (fsys *dirFS) Open(name string, flags OpenFlags, mode fs.FileMode) (File, error) {
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
	dirfd, err := openat(unix.AT_FDCWD, fsys.root, O_DIRECTORY.sysFlags(), 0)
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
	dir    *dirFile
	mutex  sync.Mutex
	refc   int32
	closed bool
	fd     int
}

func (f *dirFile) acquire() (fd int) {
	f.mutex.Lock()
	if f.closed {
		fd = -1
	} else {
		fd = f.fd
		f.refc++
	}
	f.mutex.Unlock()
	return fd
}

func (f *dirFile) release(fd int) {
	if fd >= 0 {
		f.unref()
	}
}

func (f *dirFile) ref() {
	f.mutex.Lock()
	f.refc++
	f.mutex.Unlock()
}

func (f *dirFile) unref() {
	f.mutex.Lock()
	if f.refc--; f.refc == 0 {
		f.close()
	}
	f.mutex.Unlock()
}

func (f *dirFile) close() {
	fd := f.fd
	f.fd = -1
	closeTraceError(fd)
	if f.dir != nil {
		f.dir.unref()
	}
}

func (f *dirFile) String() string {
	return fmt.Sprintf("&sandbox.dirFile{fd:%d}", f.fd)
}

func (f *dirFile) Fd() uintptr {
	return uintptr(f.fd)
}

func (f *dirFile) Close() error {
	f.mutex.Lock()
	if !f.closed {
		f.closed = true
		if f.refc--; f.refc == 0 {
			f.close()
		}
	}
	f.mutex.Unlock()
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

func (f *dirFile) openFile(name string, flags OpenFlags, mode fs.FileMode) (File, error) {
	fd, err := openat(f.fd, name, (flags | O_NOFOLLOW).sysFlags(), uint32(mode.Perm()))
	if err != nil {
		return nil, err
	}
	return newFile(f, fd), nil
}

func (f *dirFile) open(name string, flags OpenFlags, mode fs.FileMode) (File, error) {
	switch name {
	case ".":
		return f.openSelf()
	case "..":
		return f.openParent()
	default:
		return f.openFile(name, flags, mode)
	}
}

func (f *dirFile) Open(name string, flags OpenFlags, mode fs.FileMode) (File, error) {
	return withFD2(f, func(int) (File, error) {
		if fspath.IsRoot(name) {
			return f.openRoot()
		}
		if fspath.HasTrailingSlash(name) {
			flags |= O_DIRECTORY
		}
		return ResolvePath(f, name, flags.LookupFlags(), func(d *dirFile, name string) (File, error) {
			return d.open(name, flags, mode)
		})
	})
}

func (f *dirFile) Readv(iovs [][]byte) (int, error) {
	return withFD2(f, func(fd int) (int, error) { return readv(fd, iovs) })
}

func (f *dirFile) Writev(iovs [][]byte) (int, error) {
	return withFD2(f, func(fd int) (int, error) { return writev(fd, iovs) })
}

func (f *dirFile) Preadv(iovs [][]byte, offset int64) (int, error) {
	return withFD2(f, func(fd int) (int, error) { return preadv(fd, iovs, offset) })
}

func (f *dirFile) Pwritev(iovs [][]byte, offset int64) (int, error) {
	return withFD2(f, func(fd int) (int, error) { return pwritev(fd, iovs, offset) })
}

func (f *dirFile) Seek(offset int64, whence int) (int64, error) {
	return withFD2(f, func(fd int) (int64, error) { return lseek(fd, offset, whence) })
}

func (f *dirFile) Allocate(offset, length int64) error {
	return withFD1(f, func(fd int) error { return fallocate(fd, offset, length) })
}

func (f *dirFile) Truncate(size int64) error {
	return withFD1(f, func(fd int) error { return ftruncate(fd, size) })
}

func (f *dirFile) Sync() error {
	return withFD1(f, func(fd int) error { return fsync(fd) })
}

func (f *dirFile) Datasync() error {
	return withFD1(f, func(fd int) error { return fdatasync(fd) })
}

func (f *dirFile) Flags() (OpenFlags, error) {
	return withFD2(f, func(fd int) (OpenFlags, error) {
		flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
		return makeOpenFlags(flags), err
	})
}

func (f *dirFile) SetFlags(flags OpenFlags) error {
	return withFD1(f, func(fd int) error {
		_, err := unix.FcntlInt(uintptr(fd), unix.F_SETFL, flags.sysFlags())
		return err
	})
}

func (f *dirFile) ReadDirent(buf []byte) (int, error) {
	return withFD2(f, func(fd int) (int, error) {
		return ignoreEINTR2(func() (int, error) {
			return unix.ReadDirent(fd, buf)
		})
	})
}

func (f *dirFile) Stat(name string, flags LookupFlags) (FileInfo, error) {
	return resolvePath2(f, name, flags, func(fd int, name string) (FileInfo, error) {
		var stat unix.Stat_t
		var err error

		if name == "" {
			err = fstat(fd, &stat)
		} else {
			err = fstatat(fd, name, &stat, AT_SYMLINK_NOFOLLOW.sysFlags())
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
	return resolvePath2(f, name, AT_SYMLINK_NOFOLLOW, func(fd int, name string) (int, error) {
		if name == "" {
			return freadlink(fd, buf)
		} else {
			return readlinkat(fd, name, buf)
		}
	})
}

func (f *dirFile) Chtimes(name string, times [2]Timespec, flags LookupFlags) error {
	return resolvePath1(f, name, flags, func(fd int, name string) error {
		if name == "" {
			return futimens(fd, &times)
		} else {
			return utimensat(fd, name, &times, AT_SYMLINK_NOFOLLOW.sysFlags())
		}
	})
}

func (f *dirFile) Mkdir(name string, mode fs.FileMode) error {
	return resolvePath1(f, name, AT_SYMLINK_NOFOLLOW, func(fd int, name string) error {
		return mkdirat(fd, name, uint32(mode.Perm()))
	})
}

func (f *dirFile) Rmdir(name string) error {
	return resolvePath1(f, name, AT_SYMLINK_NOFOLLOW, func(fd int, name string) error {
		return unlinkat(fd, name, unix.AT_REMOVEDIR)
	})
}

func (f1 *dirFile) Rename(oldName string, newDir File, newName string) error {
	f2, ok := newDir.(*dirFile)
	if !ok {
		return EXDEV
	}
	return resolvePath1(f1, oldName, AT_SYMLINK_NOFOLLOW, func(fd1 int, name1 string) error {
		return resolvePath1(f2, newName, AT_SYMLINK_NOFOLLOW, func(fd2 int, name2 string) error {
			return renameat(fd1, name1, fd2, name2)
		})
	})
}

func (f1 *dirFile) Link(oldName string, newDir File, newName string, flags LookupFlags) error {
	f2, ok := newDir.(*dirFile)
	if !ok {
		return EXDEV
	}
	return resolvePath1(f1, oldName, flags, func(fd1 int, name1 string) error {
		return resolvePath1(f2, newName, flags, func(fd2 int, name2 string) error {
			return linkat(fd1, name1, fd2, name2, 0)
		})
	})
}

func (f *dirFile) Symlink(oldName string, newName string) error {
	return resolvePath1(f, newName, AT_SYMLINK_NOFOLLOW, func(fd int, name string) error {
		return symlinkat(oldName, fd, name)
	})
}

func (f *dirFile) Unlink(name string) error {
	return resolvePath1(f, name, AT_SYMLINK_NOFOLLOW, func(fd int, name string) error {
		return unlinkat(fd, name, 0)
	})
}

func resolvePath1(f *dirFile, name string, flags LookupFlags, do func(int, string) error) error {
	return withFD1(f, func(int) error {
		_, err := ResolvePath(f, name, flags, func(d *dirFile, name string) (_ struct{}, err error) {
			err = do(d.fd, name)
			return
		})
		return err
	})
}

func resolvePath2[R any](f *dirFile, name string, flags LookupFlags, do func(int, string) (R, error)) (R, error) {
	return withFD2(f, func(int) (R, error) {
		return ResolvePath(f, name, flags, func(d *dirFile, name string) (R, error) {
			return do(d.fd, name)
		})
	})
}

func withFD1(f *dirFile, do func(int) error) error {
	fd := f.acquire()
	if fd < 0 {
		return EBADF
	}
	defer f.release(fd)
	return do(fd)
}

func withFD2[R any](f *dirFile, do func(int) (R, error)) (R, error) {
	fd := f.acquire()
	if fd < 0 {
		var zero R
		return zero, EBADF
	}
	defer f.release(fd)
	return do(fd)
}
