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

	"github.com/stealthrocket/timecraft/internal/stream"
)

var (
	// ErrNotExist is an error value which may be tested aginst to determine
	// whether a method invocation from a Store instance failed due to being
	// called on an object which did not exist.
	ErrNotExist = fs.ErrNotExist

	// ErrReadOnly is an error returned when attempting to create an object in
	// a read-only store.
	ErrReadOnly = errors.New("read only object store")
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
	ListObjects(ctx context.Context, prefix string) stream.ReadCloser[Info]

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

// EmptyStore returns a Store instance representing an empty, read-only object
// store.
func EmptyStore() Store { return emptyStore{} }

type emptyStore struct{}

func (emptyStore) CreateObject(ctx context.Context, name string, data io.Reader) error {
	return ErrReadOnly
}

func (emptyStore) ReadObject(ctx context.Context, name string) (io.ReadCloser, error) {
	return nil, ErrNotExist
}

func (emptyStore) StatObject(ctx context.Context, name string) (Info, error) {
	return Info{}, ErrNotExist
}

func (emptyStore) ListObjects(ctx context.Context, prefix string) stream.ReadCloser[Info] {
	return emptyInfoReader{}
}

func (emptyStore) DeleteObject(ctx context.Context, name string) error {
	return nil
}

// DirStore constructs an object store from a directory entry at the given path.
//
// The function converts the directory location to an absolute path to decouple
// the store from a change of the current working directory.
//
// The directory is not created if it does not exist.
func DirStore(path string) (Store, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	return dirStore(absPath), nil
}

type dirStore string

func (store dirStore) CreateObject(ctx context.Context, name string, data io.Reader) error {
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

func (store dirStore) ReadObject(ctx context.Context, name string) (io.ReadCloser, error) {
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

func (store dirStore) StatObject(ctx context.Context, name string) (Info, error) {
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

func (store dirStore) ListObjects(ctx context.Context, prefix string) stream.ReadCloser[Info] {
	if prefix != "." && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	path, err := store.joinPath(prefix)
	if err != nil {
		return &errorInfoReader{err: err}
	}
	dir, err := os.Open(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return emptyInfoReader{}
		} else {
			return &errorInfoReader{err: err}
		}
	}
	return &dirReader{dir: dir, path: filepath.ToSlash(prefix)}
}

func (store dirStore) DeleteObject(ctx context.Context, name string) error {
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

func (store dirStore) joinPath(name string) (string, error) {
	if !fs.ValidPath(name) {
		return "", fmt.Errorf("invalid object name: %q", name)
	}
	return filepath.Join(string(store), filepath.FromSlash(name)), nil
}

type dirReader struct {
	dir  *os.File
	path string
}

func (r *dirReader) Close() error {
	r.dir.Close()
	return nil
}

func (r *dirReader) Read(items []Info) (n int, err error) {
	for n < len(items) {
		dirents, err := r.dir.ReadDir(len(items) - n)

		for _, dirent := range dirents {
			if dirent.IsDir() {
				continue
			}

			name := dirent.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}

			info, err := dirent.Info()
			if err != nil {
				r.dir.Close()
				return n, err
			}

			items[n] = Info{
				Name:      path.Join(r.path, name),
				Size:      info.Size(),
				CreatedAt: info.ModTime(),
			}
			n++
		}

		if err != nil {
			return n, err
		}
	}
	return n, nil
}

type emptyInfoReader struct{}

func (emptyInfoReader) Close() error             { return nil }
func (emptyInfoReader) Read([]Info) (int, error) { return 0, io.EOF }

type errorInfoReader struct{ err error }

func (r *errorInfoReader) Close() error             { return nil }
func (r *errorInfoReader) Read([]Info) (int, error) { return 0, r.err }
