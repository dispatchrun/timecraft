package ocifs

import (
	"errors"
	"fmt"
	"io/fs"
	"sync"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

const (
	whiteoutPrefix = ".wh."
	whiteoutOpaque = ".wh..wh..opq"
)

type file struct {
	mutex  sync.Mutex
	layers *fileLayers
	dirbuf *dirbuf
}

func newFile(layers *fileLayers) *file {
	f := &file{layers: layers}
	ref(layers)
	return f
}

func (f *file) String() string {
	l := f.ref()
	if l == nil {
		return "&ocifs.file{nil}"
	}
	defer unref(l)
	return fmt.Sprintf("&ocifs.file{layers:%v}", l.files)
}

func (f *file) ref() *fileLayers {
	f.mutex.Lock()
	layers := f.layers
	ref(layers)
	f.mutex.Unlock()
	return layers
}

func (f *file) Close() error {
	f.mutex.Lock()
	layers := f.layers
	f.layers = nil
	f.mutex.Unlock()
	unref(layers)
	return nil
}

func (f *file) openSelf() (sandbox.File, error) {
	return withLayers2(f, func(l *fileLayers) (sandbox.File, error) {
		return newFile(l), nil
	})
}

func (f *file) openParent() (sandbox.File, error) {
	return withLayers2(f, func(l *fileLayers) (sandbox.File, error) {
		if l.parent != nil {
			l = l.parent
		}
		return newFile(l), nil
	})
}

func (f *file) openRoot() (sandbox.File, error) {
	return withLayers2(f, func(l *fileLayers) (sandbox.File, error) {
		for l.parent != nil { // walk up to the root
			l = l.parent
		}
		return newFile(l), nil
	})
}

func (f *file) openFile(name string, flags sandbox.OpenFlags, mode fs.FileMode) (sandbox.File, error) {
	return withLayers2(f, func(l *fileLayers) (sandbox.File, error) {
		var files []sandbox.File
		defer func() {
			closeFiles(files) // only closed on error or panic
		}()

		whiteout := whiteoutPrefix + name

		for _, file := range l.files {
			f, err := file.Open(name, flags|sandbox.O_NOFOLLOW, mode)
			if err != nil {
				if errors.Is(err, sandbox.ELOOP) && len(files) > 0 {
					// The file was a symbolic link and it is present in a lower
					// layer which indicates that it masks all potential directories
					// below, and it is masked by the upper directory layers.
					break
				}
				if errors.Is(err, sandbox.ENOTDIR) && ((flags & sandbox.O_NOFOLLOW) != 0) && len(files) > 0 {
					// The program attempted to open a directory but a lower layer
					// had a file of a different type with the same name. This is
					// an indicator that we must stop merging layers here because
					// the file masks its lower layers and it is masked by the
					// directories at the upper layers.
					break
				}
				if !errors.Is(err, sandbox.ENOENT) {
					// Errors other than ENOENT indicate that something went wrong
					// and we must abort path resolution because we might otherwise
					// create an unconsistent view of the file system.
					return nil, err
				}
			} else {
				s, err := f.Stat("", sandbox.AT_SYMLINK_NOFOLLOW)
				if err != nil {
					f.Close()
					return nil, err
				}

				isDir := s.Mode.IsDir()
				if !isDir && len(files) > 0 {
					// Files that are not directories in lower layers cannot be
					// merged into the upper layers, they mask the layers below them
					// while also being masked by the directories above, so we stop
					// merging the views at this stage.
					f.Close()
					break
				}

				files = append(files, f)

				if !isDir {
					// This branch is taken on the first iteration, if the file is
					// not a directory it masks the underlying layers so we stop
					// merging layers here.
					break
				}
			}

			// This point is reached if the file was successfully opened, or if it
			// did not exist. We must check for whiteout files to determine whether
			// we must stop merging layers.
			if wh, err := hasWhiteout(file, whiteout); err != nil {
				return nil, err
			} else if wh {
				break
			}
		}

		if len(files) == 0 {
			// We could not find any file matching the name in any of the layers,
			// this indicates that the file does not exist.
			return nil, sandbox.ENOENT
		}

		open := newFile(&fileLayers{parent: l, files: files})
		// The new fileLayers value owns a reference to its parent
		ref(open.layers.parent)
		files = nil // prevents the defer from closing the files
		return open, nil
	})
}

func hasWhiteout(file sandbox.File, whiteout string) (has bool, err error) {
	_, err = file.Stat(whiteout, sandbox.AT_SYMLINK_NOFOLLOW)
	if err == nil {
		return true, nil
	} else if !errors.Is(err, sandbox.ENOENT) {
		return false, err
	}
	_, err = file.Stat(whiteoutOpaque, sandbox.AT_SYMLINK_NOFOLLOW)
	if err == nil {
		return true, nil
	} else if !errors.Is(err, sandbox.ENOENT) {
		return false, err
	}
	return false, nil
}

func (f *file) Open(name string, flags sandbox.OpenFlags, mode fs.FileMode) (sandbox.File, error) {
	const unsupportedFlags = sandbox.O_CREAT |
		sandbox.O_APPEND |
		sandbox.O_RDWR |
		sandbox.O_WRONLY
	return sandbox.FileOpen(f, name, flags, ^unsupportedFlags, mode,
		(*file).openRoot,
		(*file).openSelf,
		(*file).openParent,
		(*file).openFile,
	)
}

func (f *file) Stat(name string, flags sandbox.LookupFlags) (sandbox.FileInfo, error) {
	return sandbox.FileStat(f, name, flags, func(at *file, name string) (sandbox.FileInfo, error) {
		whiteout := whiteoutPrefix + name

		for _, file := range at.layers.files {
			info, err := file.Stat(name, sandbox.AT_SYMLINK_NOFOLLOW)
			if err != nil {
				if !errors.Is(err, sandbox.ENOENT) {
					return sandbox.FileInfo{}, err
				}
			} else {
				return info, err
			}

			if wh, err := hasWhiteout(file, whiteout); err != nil {
				return info, err
			} else if wh {
				break
			}
		}

		return sandbox.FileInfo{}, sandbox.ENOENT
	})
}

func (f *file) Readlink(name string, buf []byte) (int, error) {
	return sandbox.FileReadlink(f, name, func(at *file, name string) (int, error) {
		whiteout := whiteoutPrefix + name

		for _, file := range at.layers.files {
			n, err := file.Readlink(name, buf)
			if err != nil {
				if !errors.Is(err, sandbox.ENOENT) {
					return 0, err
				}
			} else {
				return n, nil
			}

			if wh, err := hasWhiteout(file, whiteout); err != nil {
				return 0, err
			} else if wh {
				break
			}
		}

		return 0, sandbox.ENOENT
	})
}

func (f *file) Fd() uintptr {
	l := f.ref()
	if l == nil {
		return ^uintptr(0)
	}
	defer unref(l)
	return l.files[0].Fd()
}

func (f *file) Readv(iovs [][]byte) (int, error) {
	return withLayers2(f, func(l *fileLayers) (int, error) {
		return l.files[0].Readv(iovs)
	})
}

func (f *file) Writev(iovs [][]byte) (int, error) {
	return 0, sandbox.EBADF
}

func (f *file) Preadv(iovs [][]byte, offset int64) (int, error) {
	return withLayers2(f, func(l *fileLayers) (int, error) {
		return l.files[0].Preadv(iovs, offset)
	})
}

func (f *file) Pwritev(iovs [][]byte, offset int64) (int, error) {
	return 0, sandbox.EBADF
}

func (f *file) CopyRange(srcOffset int64, dst sandbox.File, dstOffset int64, length int) (int, error) {
	return withLayers2(f, func(l *fileLayers) (int, error) {
		return l.files[0].CopyRange(srcOffset, dst, dstOffset, length)
	})
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return withLayers2(f, func(l *fileLayers) (int64, error) {
		f.mutex.Lock()
		d := f.dirbuf
		f.mutex.Unlock()

		if d != nil {
			if offset != 0 || whence != 0 {
				return 0, sandbox.EINVAL
			}
			d.reset()
			return 0, nil
		}
		return l.files[0].Seek(offset, whence)
	})
}

func (f *file) Allocate(int64, int64) error {
	return sandbox.EBADF
}

func (f *file) Truncate(int64) error {
	return sandbox.EBADF
}

func (f *file) Sync() error {
	return nil
}

func (f *file) Datasync() error {
	return nil
}

func (f *file) Flags() (sandbox.OpenFlags, error) {
	return 0, nil
}

func (f *file) SetFlags(sandbox.OpenFlags) error {
	return sandbox.EINVAL
}

func (f *file) ReadDirent(buf []byte) (int, error) {
	return withLayers2(f, func(l *fileLayers) (int, error) {
		f.mutex.Lock()
		if f.dirbuf == nil {
			f.dirbuf = &dirbuf{index: -1}
		}
		d := f.dirbuf
		f.mutex.Unlock()
		return d.readDirent(buf, l.files)
	})
}

func (f *file) Chtimes(string, [2]sandbox.Timespec, sandbox.LookupFlags) error {
	return sandbox.EPERM
}

func (f *file) Mkdir(string, fs.FileMode) error {
	return f.expectDirectory()
}

func (f *file) Rmdir(string) error {
	return f.expectDirectory()
}

func (f *file) Rename(string, sandbox.File, string, sandbox.RenameFlags) error {
	return f.expectDirectory()
}

func (f *file) Link(string, sandbox.File, string, sandbox.LookupFlags) error {
	return f.expectDirectory()
}

func (f *file) Symlink(string, string) error {
	return f.expectDirectory()
}

func (f *file) Unlink(string) error {
	return f.expectDirectory()
}

func (f *file) expectDirectory() error {
	return withLayers1(f, func(l *fileLayers) error {
		info, err := l.files[0].Stat("", sandbox.AT_SYMLINK_NOFOLLOW)
		if err != nil {
			return err
		}
		if !info.Mode.IsDir() {
			return sandbox.ENOTDIR
		}
		return sandbox.EROFS
	})
}

func withLayers1(f *file, do func(*fileLayers) error) error {
	layers := f.ref()
	if layers == nil {
		return sandbox.EBADF
	}
	defer unref(layers)
	return do(f.layers)
}

func withLayers2[R any](f *file, do func(*fileLayers) (R, error)) (R, error) {
	layers := f.ref()
	if layers == nil {
		var zero R
		return zero, sandbox.EBADF
	}
	defer unref(layers)
	return do(f.layers)
}
