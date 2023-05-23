package object

// Goals:
// - can store webassembly module byte code (no duplicates)
// - can store log segments
// - can find logs of processes by id
// - can be implemented by an object store (e.g. S3)
// - can store snapshots of log segments
//
// Opportunities:
// - could compact runtime configuration (no duplicates)
//
// Questions:
// - how to represent runs which are mutable?
//   * hash of manifest/metadata?
//   * one layer per segment or one file per segment?
// - should we rely more on the storage layer to model the log?

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

var (
	// ErrNotExist is an error value which may be tested aginst to determine
	// whether a method invocation from a Store instance failed due to being
	// called on an object which did not exist.
	ErrNotExist = fs.ErrNotExist
)

// Store is an interface abstracting an object storage layer.
//
// Once created, objects are immutable, the store does not need to offer a
// mechanism for updating the object content or metadata.
//
// Store instances must be safe to use concurrently from multiple goroutines.
type Store interface {
	// Creates an object with the given name, initializing its content to the
	// data read from the given io.Reader.
	//
	// The creation of objects is atomic, the store must be left unchanged if
	// an error occurs that would cause the object to be only partially created.
	CreateObject(ctx context.Context, name string, data io.Reader) error

	// Reads an existing object from the store, returning a reader exposing its
	// content.
	ReadObject(ctx context.Context, name string) (io.ReadCloser, error)

	// Retrieves information about an object in the store.
	StatObject(ctx context.Context, name string) (Info, error)

	// Lists existing objects.
	//
	// Objects that are being created by a call to CreateObject are not visible
	// until the creation completed.
	ListObjects(ctx context.Context, prefix string) Iter[Info]

	// Deletes and object from the store.
	//
	// The deletion is idempotent, deleting an object which does not exist does
	// not cause the method to return an error.
	DeleteObject(ctx context.Context, name string) error
}

// The Info type represents the set of meta data associated with an object
// within a store.
type Info struct {
	Name      string
	Size      int64
	CreatedAt time.Time
}

// DirOption represents options which may be applied when constructing an object
// store from a local directory.
type DirOption func(*dirStore)

// ReadDirBufferSize configures the step size when reading directory entries.
//
// Default to 20.
func ReadDirBufferSize(n int) DirOption {
	return DirOption(func(d *dirStore) { d.readDirBufferSize = n })
}

// DirStore constructs an object store from a directory entry at the given path.
//
// The function converts the directory location to an absolute path to decouple
// the store from a change of the current working directory.
//
// The directory is created if it does not exist. The function will fail if the
// parent directories do not exist or if a file exists at the given location.
func DirStore(path string, opts ...DirOption) (Store, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	if err := os.Mkdir(absPath, 0777); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}
	d := &dirStore{
		root:              absPath,
		readDirBufferSize: 20,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d, nil
}

type dirStore struct {
	root              string
	readDirBufferSize int
}

func (store *dirStore) CreateObject(ctx context.Context, name string, data io.Reader) error {
	filePath, err := store.joinPath(name)
	if err != nil {
		return err
	}

	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0777); err != nil {
		return err
	}

	file, err := os.CreateTemp(dirPath, "."+name+".*")
	if err != nil {
		return err
	}
	defer file.Close()
	tmpPath := file.Name()

	if _, err := io.Copy(file, data); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	return nil
}

func (store *dirStore) ReadObject(ctx context.Context, name string) (io.ReadCloser, error) {
	path, err := store.joinPath(name)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (store *dirStore) StatObject(ctx context.Context, name string) (Info, error) {
	path, err := store.joinPath(name)
	if err != nil {
		return Info{}, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return Info{}, err
	}
	info := Info{
		Name:      name,
		Size:      stat.Size(),
		CreatedAt: stat.ModTime(),
	}
	return info, nil
}

func (store *dirStore) ListObjects(ctx context.Context, prefix string) Iter[Info] {
	if prefix != "." && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	path, err := store.joinPath(prefix)
	if err != nil {
		return Err[Info](err)
	}
	dir, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Empty[Info]()
		}
		return Err[Info](err)
	}
	return &dirIter{
		dir:  dir,
		path: filepath.ToSlash(prefix),
		n:    store.readDirBufferSize,
	}
}

func (store *dirStore) DeleteObject(ctx context.Context, name string) error {
	path, err := store.joinPath(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	// TODO: cleanup empty parent directories
	return nil
}

func (store *dirStore) joinPath(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", fmt.Errorf("invalid object name: %q", name)
	}
	return filepath.Join(store.root, filepath.FromSlash(name)), nil
}

type dirIter struct {
	dir  *os.File
	next []fs.DirEntry
	path string
	info Info
	err  error
	n    int
}

func (it *dirIter) Close() error {
	it.dir.Close()
	it.next = nil
	return it.err
}

func (it *dirIter) Next() bool {
	for {
		if it.err != nil {
			return false
		}

		if it.dir == nil {
			return false
		}

		for len(it.next) > 0 {
			dirent := it.next[0]
			it.next = it.next[1:]

			if dirent.IsDir() {
				continue
			}

			name := dirent.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}

			info, err := dirent.Info()
			if err != nil {
				it.err = err
				return false
			}

			it.info.Name = path.Join(it.path, name)
			it.info.Size = info.Size()
			it.info.CreatedAt = info.ModTime()
			return true
		}

		entries, err := it.dir.ReadDir(it.n)
		if len(entries) > 0 {
			it.next = entries
		} else {
			if err == io.EOF {
				err = nil
			}
			it.Close()
			it.err = err
			return false
		}
	}
}

func (it *dirIter) Item() Info {
	return it.info
}
