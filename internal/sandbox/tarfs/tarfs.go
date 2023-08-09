package tarfs

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"time"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/fspath"
	"golang.org/x/exp/slices"
)

// FileSystem is an implementation of the sandbox.FileSystem interface backed by
// a tarball.
type FileSystem struct {
	data     io.ReaderAt
	size     int64
	memsize  int64
	filesize int64
	root     dir
}

// Open satisfies sandbox.FileSystem.
func (fsys *FileSystem) Open(name string, flags int, mode fs.FileMode) (sandbox.File, error) {
	f, err := fsys.root.open(fsys)
	if err != nil {
		return nil, err
	}
	if fspath.IsRoot(name) {
		return f, nil
	}
	defer f.Close()
	return f.Open(name, flags, mode)
}

// Size returns the size of data in the file system, which is the value passed
// to Open when fsys was created. It is the full size of the underlying tarball.
func (fsys *FileSystem) Size() int64 {
	return fsys.size
}

// Memsize returns the size occupied by the file system in memory.
func (fsys *FileSystem) Memsize() int64 {
	return fsys.memsize
}

// Filesize returns the size occupied by the file data in the file system.
func (fsys *FileSystem) Filesize() int64 {
	return fsys.filesize
}

// Open creates a file system from the tarball represented by the given section.
//
// The file system retains the io.ReaderAt as a backing storage layer, it does
// not load the content of the files present in the tarball in memory; only the
// structure of the file system is held in memory (e.g. directory paths and
// metadata about the files).
func Open(data io.ReaderAt, size int64) (*FileSystem, error) {
	section := io.NewSectionReader(data, 0, size)
	r := tar.NewReader(section)

	modTime := time.Now()
	fsys := &FileSystem{
		data: data,
		size: size,
		root: makeDir(modTime),
	}

	links := make(map[string]string)
	files := make(map[string]fileEntry)
	files["/"] = &fsys.root

	for {
		header, err := r.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		name := absPath(header.Name)

		switch name {
		case ".", "..":
			continue // ignore those entries, they should never exist anyway
		case "/":
			template := newDir(header)
			fsys.root.perm = template.perm
			fsys.root.mtime = template.mtime
			fsys.root.atime = template.atime
			fsys.root.ctime = template.ctime
			continue
		}

		var entry fileEntry
		switch header.Typeflag {
		case tar.TypeReg:
			offset, _ := section.Seek(0, io.SeekCurrent)
			entry = newFile(header, offset)
		case tar.TypeDir:
			entry = newDir(header)
		case tar.TypeSymlink:
			entry = newSymlink(header)
		case tar.TypeLink:
			if _, exists := links[name]; exists {
				return nil, fmt.Errorf("%s: duplicate link entry in tar archive", name)
			}
			links[name] = absPath(header.Linkname)
			continue
		default:
			entry = newPlaceholder(header)
		}
		if files[name] != nil {
			return nil, fmt.Errorf("%s: duplicate file entry in tar archive", name)
		}
		files[name] = entry

		if err := makePath(files, name, modTime, entry); err != nil {
			return nil, err
		}
	}

	for link, name := range links {
		entry := files[name]
		switch f := entry.(type) {
		case *file:
			f.nlink++
		case *symlink:
			f.nlink++
		default:
			return nil, fmt.Errorf("%s->%s: hard link to invalid file in tar archive", link, name)
		}
		if files[link] != nil {
			return nil, fmt.Errorf("%s: duplicate file entry in tar archive", link)
		}
		if err := makePath(files, link, modTime, entry); err != nil {
			return nil, err
		}
	}

	for name, entry := range files {
		if d, ok := entry.(*dir); ok {
			d.parent = files[path.Dir(name)].(*dir)
			slices.SortFunc(d.ents, func(a, b dirEntry) bool {
				return a.name < b.name
			})
		}
	}

	for _, entry := range files {
		fsys.memsize += int64(entry.memsize())
		if f, ok := entry.(*file); ok {
			fsys.filesize += f.size
		}
	}

	return fsys, nil
}

// OpenFile is like Open but it takes an *os.File opened on a tarball as
// argument.
//
// The file must remain open for as long as the program needs to access the file
// system.
func OpenFile(f *os.File) (*FileSystem, error) {
	s, err := f.Stat()
	if err != nil {
		return nil, err
	}
	return Open(f, s.Size())
}

type fileEntry interface {
	open(fsys *FileSystem) (sandbox.File, error)

	stat() sandbox.FileInfo

	mode() fs.FileMode

	memsize() uintptr
}

func absPath(p string) string {
	return path.Join("/", p)
}

func makePath(files map[string]fileEntry, name string, modTime time.Time, file fileEntry) error {
	var d *dir

	dirname := path.Dir(name)
	switch f := files[dirname].(type) {
	case nil:
		if err := makePath(files, name, modTime, file); err != nil {
			return err
		}
		dir := makeDir(modTime)
		d = &dir
		files[dirname] = d
	case *dir:
		d = f
	default:
		return sandbox.EPERM
	}

	d.ents = append(d.ents, dirEntry{
		name: path.Base(name),
		file: file,
	})
	return nil
}

type readOnlyFile struct{}

func (readOnlyFile) Fd() uintptr { return ^uintptr(0) }

func (readOnlyFile) Writev([][]byte) (int, error) { return 0, sandbox.EBADF }

func (readOnlyFile) Pwritev([][]byte, int64) (int, error) { return 0, sandbox.EBADF }

func (readOnlyFile) Allocate(int64, int64) error { return sandbox.EBADF }

func (readOnlyFile) Truncate(int64) error { return sandbox.EBADF }

func (readOnlyFile) Sync() error { return nil }

func (readOnlyFile) Datasync() error { return nil }

func (readOnlyFile) Flags() (int, error) { return 0, nil }

func (readOnlyFile) SetFlags(int) error { return sandbox.EINVAL }

func (readOnlyFile) Chtimes(string, [2]sandbox.Timespec, int) error { return sandbox.EPERM }

type leafFile struct{ readOnlyFile }

func (leafFile) Open(string, int, fs.FileMode) (sandbox.File, error) { return nil, sandbox.ENOTDIR }

func (leafFile) ReadDirent([]byte) (int, error) { return 0, sandbox.ENOTDIR }

func (leafFile) Mkdir(string, fs.FileMode) error { return sandbox.ENOTDIR }

func (leafFile) Rmdir(string) error { return sandbox.ENOTDIR }

func (leafFile) Rename(string, sandbox.File, string) error { return sandbox.ENOTDIR }

func (leafFile) Link(string, sandbox.File, string, int) error { return sandbox.ENOTDIR }

func (leafFile) Symlink(string, string) error { return sandbox.ENOTDIR }

func (leafFile) Unlink(string) error { return sandbox.ENOTDIR }
