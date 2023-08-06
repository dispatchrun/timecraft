package tarfs

import (
	"io/fs"
	"path"
	"sort"
	"sync"
	"sync/atomic"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

type dir struct {
	name string
	ents []dirEntry
	info sandbox.FileInfo
}

type dirEntry struct {
	name string
	file fileEntry
}

func (d *dir) open(fsys *fileSystem) (sandbox.File, error) {
	open := &openDir{fsys: fsys}
	open.dir.Store(d)
	return open, nil
}

func (d *dir) stat() sandbox.FileInfo {
	return d.info
}

func (d *dir) mode() fs.FileMode {
	return d.info.Mode
}

func (d *dir) find(name string) fileEntry {
	i := sort.Search(len(d.ents), func(i int) bool {
		return d.ents[i].name >= name
	})
	if i == len(d.ents) || d.ents[i].name != name {
		return nil
	}
	return d.ents[i].file
}

func resolve[R any](fsys *fileSystem, cwd *dir, name string, flags int, do func(fileEntry) (R, error)) (R, error) {
	var zero R

	for loop := 0; loop < sandbox.MaxFollowSymlink; loop++ {
		if name == "" {
			return do(cwd)
		}

		var elem string
		elem, name = splitPath(name)

		if elem == "/" {
			cwd = &fsys.root
			continue
		}

		f := cwd.find(elem)
		if f == nil {
			return zero, sandbox.ENOENT
		}

		if name != "" {
			switch c := f.(type) {
			case *symlink:
				name = path.Join(c.link, name)
			case *dir:
				cwd = c
			default:
				return zero, sandbox.ENOTDIR
			}
			continue
		}

		if (flags & sandbox.O_DIRECTORY) != 0 {
			if _, ok := f.(*dir); !ok {
				return zero, sandbox.ENOTDIR
			}
		}

		if (flags & sandbox.O_NOFOLLOW) == 0 {
			if s, ok := f.(*symlink); ok {
				name = s.link
				continue
			}
		}

		return do(f)
	}

	return zero, sandbox.ELOOP
}

type openDir struct {
	readOnlyFile
	fsys   *fileSystem
	dir    atomic.Pointer[dir]
	mu     sync.Mutex
	index  int
	offset uint64
}

func (d *openDir) Name() string {
	if dir := d.dir.Load(); dir != nil {
		return dir.name
	}
	return ""
}

func (d *openDir) Close() error {
	d.dir.Store(nil)
	return nil
}

func (d *openDir) Open(name string, flags int, mode fs.FileMode) (sandbox.File, error) {
	const unsupportedFlags = sandbox.O_CREAT |
		sandbox.O_APPEND |
		sandbox.O_RDWR |
		sandbox.O_WRONLY

	if ((flags & unsupportedFlags) != 0) || mode != 0 || name == "" {
		return nil, sandbox.EINVAL
	}

	dir := d.dir.Load()
	if dir == nil {
		return nil, sandbox.EBADF
	}
	return resolve(d.fsys, dir, name, flags, func(f fileEntry) (sandbox.File, error) {
		if _, ok := f.(*symlink); ok {
			return nil, sandbox.ELOOP
		}
		return f.open(d.fsys)
	})
}

func (d *openDir) Stat(name string, flags int) (sandbox.FileInfo, error) {
	dir := d.dir.Load()
	if dir == nil {
		return sandbox.FileInfo{}, sandbox.EBADF
	}
	openFlags := 0
	if (flags & sandbox.AT_SYMLINK_NOFOLLOW) != 0 {
		openFlags |= sandbox.O_NOFOLLOW
	}
	return resolve(d.fsys, dir, name, openFlags, func(f fileEntry) (sandbox.FileInfo, error) {
		return f.stat(), nil
	})
}

func (d *openDir) Readlink(name string, buf []byte) (int, error) {
	dir := d.dir.Load()
	if dir == nil {
		return 0, sandbox.EBADF
	}
	return resolve(d.fsys, dir, name, sandbox.O_NOFOLLOW, func(f fileEntry) (int, error) {
		if s, ok := f.(*symlink); ok {
			return copy(buf, s.link), nil
		} else {
			return 0, sandbox.EINVAL
		}
	})
}

func (d *openDir) Seek(offset int64, whence int) (int64, error) {
	if d.dir.Load() == nil {
		return 0, sandbox.EBADF
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// For now we only support resetting the directory reader to the start
	// of the directory entry list.
	if offset != 0 || whence != 0 {
		return 0, sandbox.EINVAL
	}

	d.index, d.offset = 0, 0
	return 0, nil
}

func (d *openDir) ReadDirent(buf []byte) (int, error) {
	dir := d.dir.Load()
	if dir == nil {
		return 0, sandbox.EBADF
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	n := 0
	for n < len(buf) && d.index < len(dir.ents) {
		dirent := &dir.ents[d.index]
		wn := sandbox.WriteDirent(buf[n:], dirent.file.mode(), 0, d.offset, dirent.name)
		n += wn
		d.index++
		d.offset += uint64(n)
	}
	return n, nil
}

func (*openDir) Mkdir(string, fs.FileMode) error { return sandbox.EROFS }

func (*openDir) Rmdir(string) error { return sandbox.EROFS }

func (*openDir) Rename(string, sandbox.File, string) error { return sandbox.EROFS }

func (*openDir) Link(string, sandbox.File, string, int) error { return sandbox.EROFS }

func (*openDir) Symlink(string, string) error { return sandbox.EROFS }

func (*openDir) Unlink(string) error { return sandbox.EROFS }

func (*openDir) Readv([][]byte) (int, error) { return 0, sandbox.EISDIR }

func (*openDir) Preadv([][]byte, int64) (int, error) { return 0, sandbox.EISDIR }
