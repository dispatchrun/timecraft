package overlayfs

import (
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

const (
	whiteoutPrefix = ".wh."
	tempFilePrefix = ".wh..tmp."
)

type file struct {
	mutex   sync.Mutex
	overlay *fileOverlay
	dir     *dirOverlay
}

func newFile(overlay *fileOverlay) *file {
	f := &file{overlay: overlay}
	ref(overlay)
	return f
}

func (f *file) String() string {
	overlay := f.ref()
	if overlay == nil {
		return "&overlayfs.file{nil}"
	}
	defer unref(overlay)
	return fmt.Sprintf("&overlayfs.file{lower:%v,upper:%v}", overlay.lower, overlay.upper)
}

func (f *file) ref() *fileOverlay {
	f.mutex.Lock()
	overlay := f.overlay
	ref(overlay)
	f.mutex.Unlock()
	return overlay
}

func (f *file) Close() error {
	f.mutex.Lock()
	overlay := f.overlay
	f.overlay = nil
	f.mutex.Unlock()
	unref(overlay)
	return nil
}

func (f *file) openSelf() (sandbox.File, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (sandbox.File, error) {
		return newFile(overlay), nil
	})
}

func (f *file) openParent() (sandbox.File, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (sandbox.File, error) {
		if overlay.parent != nil {
			overlay = overlay.parent
		}
		return newFile(overlay), nil
	})
}

func (f *file) openRoot() (sandbox.File, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (sandbox.File, error) {
		for overlay.parent != nil {
			overlay = overlay.parent
		}
		return newFile(overlay), nil
	})
}

func (f *file) openFile(name string, flags sandbox.OpenFlags, mode fs.FileMode) (sandbox.File, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (sandbox.File, error) {
		const writeFlags = sandbox.O_CREAT |
			sandbox.O_APPEND |
			sandbox.O_RDWR |
			sandbox.O_WRONLY |
			sandbox.O_TRUNC

		hasWhiteout := false
		if overlay.lower != nil && overlay.upper != nil {
			if ok, err := exists(overlay.upper, whiteoutPrefix+name); err != nil {
				return nil, err
			} else {
				hasWhiteout = ok
			}
		}

		if overlay.lower != nil && !hasWhiteout {
			// When the file is going to be created, we must ensure that the
			// path exists in the upper layer if it exists in the lower layer.
			if (flags & writeFlags) != 0 {
				// If the file is to be created exclusively, we must check that
				// it does not exist in the lower layer, otherwise it might be
				// created in the upper loyer because it does not exist there.
				if (flags & sandbox.O_EXCL) != 0 {
					if ok, err := exists(overlay.lower, name); err != nil {
						return nil, err
					} else if ok {
						return nil, sandbox.EEXIST
					}
				}
				if err := overlay.makePath(); err != nil {
					return nil, err
				}
			}
			// When the file is open for writing, we need to move it to the
			// upper layer to allow it to receive writes. However, if the file
			// is to be truncated, we don't need to create its content and can
			// simply let the upper layer add an empty file.
			if ((flags & writeFlags) != 0) && ((flags & sandbox.O_TRUNC) == 0) {
				if err := overlay.makeFile(name); err != nil {
					return nil, err
				}
			}
		}

		var lower sandbox.File
		var upper sandbox.File
		var err error

		defer func() {
			closeIfNotNil(lower)
			closeIfNotNil(upper)
		}()

		if overlay.upper != nil {
			upper, err = overlay.upper.Open(name, flags, mode)
			if err != nil {
				if !errors.Is(err, sandbox.ENOENT) {
					return nil, err
				}
			}
		}

		if overlay.lower != nil && !hasWhiteout {
			const readOnlyFlags = sandbox.O_DIRECTORY | sandbox.O_NOFOLLOW | sandbox.O_RDONLY
			lower, err = overlay.lower.Open(name, flags&readOnlyFlags, 0)
			if err != nil {
				if !errors.Is(err, sandbox.ENOENT) && upper == nil {
					return nil, err
				}
			}
		}

		if lower == nil && upper == nil {
			return nil, sandbox.ENOENT
		}

		open := newFile(newFileOverlay(overlay, lower, upper, name))
		lower = nil
		upper = nil
		return open, nil
	})
}

func lstat(file sandbox.File, name string) (sandbox.FileInfo, error) {
	return file.Stat(name, sandbox.AT_SYMLINK_NOFOLLOW)
}

func exists(file sandbox.File, name string) (has bool, err error) {
	_, err = lstat(file, name)
	if err == nil {
		return true, nil
	} else if !errors.Is(err, sandbox.ENOENT) {
		return false, err
	} else {
		return false, nil
	}
}

func (f *file) Open(name string, flags sandbox.OpenFlags, mode fs.FileMode) (sandbox.File, error) {
	return sandbox.FileOpen(f, name, flags, ^sandbox.OpenFlags(0), mode,
		(*file).openRoot,
		(*file).openSelf,
		(*file).openParent,
		(*file).openFile,
	)
}

func (f *file) Stat(name string, flags sandbox.LookupFlags) (sandbox.FileInfo, error) {
	return sandbox.FileStat(f, name, flags, func(at *file, name string) (sandbox.FileInfo, error) {
		return withOverlay2(at, func(overlay *fileOverlay) (sandbox.FileInfo, error) {
			whiteout := whiteoutPrefix + name

			for i, file := range overlay.files() {
				info, err := file.Stat(name, sandbox.AT_SYMLINK_NOFOLLOW)
				if err != nil {
					if !errors.Is(err, sandbox.ENOENT) {
						return info, err
					}
				} else {
					// TOOD: directories in the upper layer cannot be made
					// unwritable at this time, so we always set their write
					// permissions to represent this limitation.
					//
					// See (*fileOverlay).makePath for more details.
					if info.Mode.Type() == fs.ModeDir {
						info.Mode = fs.ModeDir | 0755
					}
					// TOOD: hard links in lower layers are not supported
					// because we would need to maintain the link entries when
					// migrating a file from the lower to the upper layer, and
					// this requires scanning the entire file system to find
					// other links, so we pretend that there are no hard links
					// in the lower layers and each entry points to a unique
					// file (since the lower layers are read-only, it works).
					//
					// A potential solution would be to construct an in-memory
					// view of the file system which maintains the relationships
					// between directory entries using inodes as deduplication
					// keys. However, the tarfs layers do not expose inodes,
					// they are all zero, so we cannot implement it for now.
					//
					// Since it is unclear where WASI will land with regard to
					// hard links, we simply disable them in the lower layers
					// and will address in the future when needed.
					if i > 0 {
						info.Nlink = 1
					}
					return info, nil
				}

				if ok, err := exists(file, whiteout); err != nil {
					return sandbox.FileInfo{}, err
				} else if ok {
					break
				}
			}

			return sandbox.FileInfo{}, sandbox.ENOENT
		})
	})
}

func (f *file) Readlink(name string, buf []byte) (int, error) {
	return sandbox.FileReadlink(f, name, func(at *file, name string) (int, error) {
		return withOverlay2(at, func(overlay *fileOverlay) (int, error) {
			whiteout := whiteoutPrefix + name

			for _, file := range overlay.files() {
				n, err := file.Readlink(name, buf)
				if err != nil {
					if !errors.Is(err, sandbox.ENOENT) {
						return 0, err
					}
				} else {
					return n, nil
				}

				if ok, err := exists(file, whiteout); err != nil {
					return 0, err
				} else if ok {
					break
				}
			}

			return 0, sandbox.ENOENT
		})
	})
}

func (f *file) Fd() uintptr {
	overlay := f.ref()
	if overlay == nil {
		return ^uintptr(0)
	}
	defer unref(overlay)
	return overlay.lower.Fd()
}

func (f *file) Readv(iovs [][]byte) (int, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (int, error) {
		return overlay.file().Readv(iovs)
	})
}

func (f *file) Writev(iovs [][]byte) (int, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (int, error) {
		// This condition will be true when the file was not opened with a write
		// flag like O_WRONLY or O_RDWR.
		if overlay.upper == nil {
			return 0, sandbox.EBADF
		}
		return overlay.upper.Writev(iovs)
	})
}

func (f *file) Preadv(iovs [][]byte, offset int64) (int, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (int, error) {
		return overlay.file().Preadv(iovs, offset)
	})
}

func (f *file) Pwritev(iovs [][]byte, offset int64) (int, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (int, error) {
		// This condition will be true when the file was not opened with a write
		// flag like O_WRONLY or O_RDWR.
		if overlay.upper == nil {
			return 0, sandbox.EBADF
		}
		return overlay.upper.Pwritev(iovs, offset)
	})
}

func (f *file) CopyRange(srcOffset int64, dst sandbox.File, dstOffset int64, length int) (int, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (int, error) {
		return overlay.file().CopyRange(srcOffset, dst, dstOffset, length)
	})
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (int64, error) {
		f.mutex.Lock()
		dir := f.dir
		f.mutex.Unlock()

		if dir != nil {
			if offset != 0 || whence != 0 {
				return 0, sandbox.EINVAL
			}
			dir.reset()
			return 0, nil
		}

		return overlay.file().Seek(offset, whence)
	})
}

func (f *file) Allocate(int64, int64) error {
	return withOverlay1(f, func(overlay *fileOverlay) error {
		return sandbox.ENOSYS
	})
}

func (f *file) Truncate(int64) error {
	return withOverlay1(f, func(overlay *fileOverlay) error {
		return sandbox.ENOSYS
	})
}

func (f *file) Sync() error {
	return withOverlay1(f, func(overlay *fileOverlay) error {
		return sandbox.ENOSYS
	})
}

func (f *file) Datasync() error {
	return withOverlay1(f, func(overlay *fileOverlay) error {
		return sandbox.ENOSYS
	})
}

func (f *file) Flags() (sandbox.OpenFlags, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (sandbox.OpenFlags, error) {
		return overlay.file().Flags()
	})
}

func (f *file) SetFlags(sandbox.OpenFlags) error {
	return withOverlay1(f, func(overlay *fileOverlay) error {
		return sandbox.ENOSYS
	})
}

func (f *file) ReadDirent(buf []byte) (int, error) {
	return withOverlay2(f, func(overlay *fileOverlay) (int, error) {
		lower := overlay.lower
		upper := overlay.upper
		switch {
		case lower == nil:
			return upper.ReadDirent(buf)
		case upper == nil:
			return lower.ReadDirent(buf)
		default:
			f.mutex.Lock()
			if f.dir == nil {
				f.dir = newDirOverlay()
			}
			f.mutex.Unlock()
			return f.dir.readDirent(lower, upper, buf)
		}
	})
}

func (f *file) Chtimes(name string, times [2]sandbox.Timespec, flags sandbox.LookupFlags) error {
	return resolveUpperPath(f, name, flags, func(upper sandbox.File, name string) error {
		return upper.Chtimes(name, times, flags)
	})
}

func (f *file) Mkdir(name string, perm fs.FileMode) error {
	return resolveUpperPath(f, name, sandbox.AT_SYMLINK_NOFOLLOW, func(upper sandbox.File, name string) error {
		return upper.Mkdir(name, perm)
	})
}

func (f *file) Rmdir(name string) error {
	return resolveUpperPath(f, name, sandbox.AT_SYMLINK_NOFOLLOW, func(upper sandbox.File, name string) error {
		return upper.Rmdir(name)
	})
}

func (f1 *file) Rename(oldName string, newFile sandbox.File, newName string, flags sandbox.RenameFlags) error {
	f2, ok := newFile.(*file)
	if !ok {
		return sandbox.EXDEV
	}
	const lookup = sandbox.AT_SYMLINK_NOFOLLOW
	return resolveUpperPath(f1, oldName, lookup, func(upper1 sandbox.File, name1 string) error {
		return resolveUpperPath(f2, newName, lookup, func(upper2 sandbox.File, name2 string) error {
			return upper1.Rename(name1, upper2, name2, flags)
		})
	})
}

func (f1 *file) Link(oldName string, newFile sandbox.File, newName string, flags sandbox.LookupFlags) error {
	f2, ok := newFile.(*file)
	if !ok {
		return sandbox.EXDEV
	}
	return resolveUpperPath(f1, oldName, flags, func(upper1 sandbox.File, name1 string) error {
		return resolveUpperPath(f2, newName, flags, func(upper2 sandbox.File, name2 string) error {
			return upper1.Link(name1, upper2, name2, flags)
		})
	})
}

func (f *file) Symlink(oldName, newName string) error {
	return resolveUpperPath(f, newName, sandbox.AT_SYMLINK_NOFOLLOW, func(upper sandbox.File, name string) error {
		return upper.Symlink(oldName, name)
	})
}

func (f *file) Unlink(name string) error {
	return resolveOverlayPath(f, name, sandbox.AT_SYMLINK_NOFOLLOW, func(overlay *fileOverlay, name string) error {
		// When the file does not exist in the lower layer we should simply
		// remove it from the upper layer.
		if overlay.lower == nil {
			return overlay.upper.Unlink(name)
		}

		// If the file exists on the lower layer, we must create a whiteout file
		// to mask it in the upper layer.
		s, err := overlay.lower.Stat(name, sandbox.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			if !errors.Is(err, sandbox.ENOENT) {
				return err
			}
			return overlay.upper.Unlink(name)
		}

		// If a whiteout file already exists, this indicates that the lower
		// layer was already masked, we can simply delegate to unlinking the
		// entry in the upper layer.
		whiteout := whiteoutPrefix + name

		// If the file is a directory, Rmdir should be used instead. However,
		// we must make an exception and ignore this condition if the whiteout
		// file already exists (this coordinates with Rmdir).
		if s.Mode.IsDir() {
			if ok, err := exists(overlay.upper, whiteout); err != nil {
				return err
			} else if !ok {
				return sandbox.EISDIR
			}
		}

		// Always start by creating the whiteout file; this ensures that we
		// atomically mask the file in the lower layer.
		wh, err := overlay.upper.Open(whiteout, sandbox.O_CREAT|sandbox.O_EXCL|sandbox.O_TRUNC, 0666)
		if err != nil {
			if !errors.Is(err, sandbox.EEXIST) {
				return err
			}
		} else {
			wh.Close()
		}

		// Now there are two conditions: either the file existed in the upper
		// layer and it can be unlinked, which causes it to disappear from all
		// layers, or it did not exist and in order to successfully unlink it
		// from the lower layer the current operation must have created the
		// whiteout file.
		if err := overlay.upper.Unlink(name); err != nil {
			if !errors.Is(err, sandbox.ENOENT) || wh == nil {
				return err
			}
		}

		return nil
	})
}

func resolveOverlayPath(f *file, name string, flags sandbox.LookupFlags, do func(*fileOverlay, string) error) error {
	_, err := sandbox.ResolvePath(f, name, flags, func(d *file, name string) (_ struct{}, err error) {
		err = withOverlay1(d, func(overlay *fileOverlay) error {
			if overlay.lower != nil {
				if err := overlay.makePath(); err != nil {
					return err
				}
			}
			return do(overlay, name)
		})
		return
	})
	return err
}

func resolveUpperPath(f *file, name string, flags sandbox.LookupFlags, do func(sandbox.File, string) error) error {
	return resolveOverlayPath(f, name, flags, func(overlay *fileOverlay, name string) error {
		return do(overlay.upper, name)
	})
}

func withOverlay1(f *file, do func(*fileOverlay) error) error {
	overlay := f.ref()
	if overlay == nil {
		return sandbox.EBADF
	}
	defer unref(overlay)
	return do(overlay)
}

func withOverlay2[R any](f *file, do func(*fileOverlay) (R, error)) (R, error) {
	overlay := f.ref()
	if overlay == nil {
		var zero R
		return zero, sandbox.EBADF
	}
	defer unref(overlay)
	return do(overlay)
}
