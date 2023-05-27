package object

import (
	"bytes"
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

	"golang.org/x/exp/slices"

	"github.com/stealthrocket/timecraft/internal/object/query"
	"github.com/stealthrocket/timecraft/internal/stream"
)

var (
	// ErrNotExist is an error value which may be tested against to determine
	// whether a method invocation from a Store instance failed due to being
	// called on an object which did not exist.
	ErrNotExist = fs.ErrNotExist

	// ErrReadOnly is an error returned when attempting to create an object in
	// a read-only store.
	ErrReadOnly = errors.New("read only object store")
)

// Tag represents a name/value pair attached to an object.
type Tag struct {
	Name  string
	Value string
}

func AppendTags(buf []byte, tags ...Tag) []byte {
	for _, tag := range tags {
		buf = append(buf, tag.Name...)
		buf = append(buf, '=')
		buf = append(buf, tag.Value...)
		buf = append(buf, '\n')
	}
	return buf
}

func (t Tag) String() string {
	return t.Name + "=" + t.Value
}

// Filter represents a predicate applicated to objects to determine whether they
// are part of the result of a ListObject operation.
type Filter = query.Filter[*Info]

func AFTER(t time.Time) Filter { return query.After[*Info](t) }

func BEFORE(t time.Time) Filter { return query.Before[*Info](t) }

func MATCH(name, value string) Filter { return query.Match[*Info]{name, value} }

func AND(f1, f2 Filter) Filter { return query.And[*Info]{f1, f2} }

func OR(f1, f2 Filter) Filter { return query.Or[*Info]{f1, f2} }

func NOT(f Filter) Filter { return query.Not[*Info]{f} }

func validTag(tag Tag) bool {
	return validTagName(tag.Name) && validTagValue(tag.Value)
}

func validTagName(name string) bool {
	return name != "" &&
		strings.IndexByte(name, '=') < 0 &&
		strings.IndexByte(name, '\n') < 0
}

func validTagValue(value string) bool {
	return strings.IndexByte(value, '\n') < 0
}

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
	CreateObject(ctx context.Context, name string, data io.Reader, tags ...Tag) error

	// Reads an existing object from the store, returning a reader exposing its
	// content.
	ReadObject(ctx context.Context, name string) (io.ReadCloser, error)

	// Retrieves information about an object in the store.
	StatObject(ctx context.Context, name string) (Info, error)

	// Lists existing objects.
	//
	// Objects that are being created by a call to CreateObject are not visible
	// until the creation completed.
	ListObjects(ctx context.Context, prefix string, filters ...Filter) stream.ReadCloser[Info]

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
	Tags      []Tag
}

func (info *Info) After(t time.Time) bool {
	return info.CreatedAt.After(t)
}

func (info *Info) Before(t time.Time) bool {
	return info.CreatedAt.Before(t)
}

func (info *Info) Match(name, value string) bool {
	for _, tag := range info.Tags {
		if tag.Name == name && tag.Value == value {
			return true
		}
	}
	return false
}

func (info *Info) Lookup(name string) (string, bool) {
	for _, tag := range info.Tags {
		if tag.Name == name {
			return tag.Value, true
		}
	}
	return "", false
}

// EmptyStore returns a Store instance representing an empty, read-only object
// store.
func EmptyStore() Store { return emptyStore{} }

type emptyStore struct{}

func (emptyStore) CreateObject(context.Context, string, io.Reader, ...Tag) error {
	return ErrReadOnly
}

func (emptyStore) ReadObject(context.Context, string) (io.ReadCloser, error) {
	return nil, ErrNotExist
}

func (emptyStore) StatObject(context.Context, string) (Info, error) {
	return Info{}, ErrNotExist
}

func (emptyStore) ListObjects(context.Context, string, ...Filter) stream.ReadCloser[Info] {
	return emptyInfoReader{}
}

func (emptyStore) DeleteObject(context.Context, string) error {
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

func (store dirStore) CreateObject(ctx context.Context, name string, data io.Reader, tags ...Tag) error {
	filePath, err := store.joinPath(name)
	if err != nil {
		return err
	}

	dirPath, fileName := filepath.Split(filePath)
	if strings.HasPrefix(fileName, ".") {
		return fmt.Errorf("object names cannot start with a dot: %s", name)
	}

	if err := os.MkdirAll(dirPath, 0777); err != nil {
		return err
	}

	tagsPath := ""
	tagsData := []byte(nil)
	if len(tags) > 0 {
		for _, tag := range tags {
			if !validTag(tag) {
				return fmt.Errorf("invalid tag: %q=%q", tag.Name, tag.Value)
			}
		}

		tagsPath = filepath.Join(dirPath, ".tags", fileName)
		tagsData = make([]byte, 0, 256)
		tagsData = AppendTags(tagsData, tags...)

		if err := os.Mkdir(filepath.Join(dirPath, ".tags"), 0777); err != nil {
			if !errors.Is(err, fs.ErrExist) {
				return err
			}
		}
		if err := os.WriteFile(tagsPath, tagsData, 0666); err != nil {
			return err
		}
	}

	objectFile, err := os.CreateTemp(dirPath, "."+fileName+".*")
	if err != nil {
		return err
	}
	defer objectFile.Close()

	tmpPath := objectFile.Name()
	success := false
	defer func() {
		if !success {
			os.Remove(tmpPath)
			if tagsPath != "" {
				os.Remove(tagsPath)
			}
		}
	}()

	if _, err := io.Copy(objectFile, data); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, filePath); err != nil {
		return err
	}

	success = true
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
	stat, err := os.Lstat(path)
	if err != nil {
		return Info{}, err
	}
	if !stat.Mode().IsRegular() {
		return Info{}, ErrNotExist
	}
	dir, base := filepath.Split(path)
	tags, err := readTags(filepath.Join(dir, ".tags", base))
	if err != nil {
		return Info{}, err
	}
	info := Info{
		Name:      name,
		Size:      stat.Size(),
		CreatedAt: stat.ModTime(),
		Tags:      tags,
	}
	return info, nil
}

func (store dirStore) ListObjects(ctx context.Context, prefix string, filters ...Filter) stream.ReadCloser[Info] {
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
	return &dirReader{
		dir:     dir,
		path:    path,
		prefix:  prefix,
		filters: slices.Clone(filters),
	}
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
	if name = path.Clean(name); !fs.ValidPath(name) {
		return "", fmt.Errorf("invalid object name: %q", name)
	}
	return filepath.Join(string(store), filepath.FromSlash(name)), nil
}

func readTags(path string) ([]Tag, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			err = nil
		}
		return nil, err
	}
	tags := make([]Tag, 0, 8)
	nl := []byte{'\n'}
	eq := []byte{'='}
	for _, line := range bytes.Split(b, nl) {
		name, value, ok := bytes.Cut(line, eq)
		if !ok {
			continue
		}
		tags = append(tags, Tag{
			Name:  string(name),
			Value: string(value),
		})
	}
	return tags, nil
}

type dirReader struct {
	dir     *os.File
	path    string
	prefix  string
	filters []Filter
}

func (r *dirReader) Close() error {
	r.dir.Close()
	return nil
}

func (r *dirReader) Read(items []Info) (n int, err error) {
	for n < len(items) {
		dirents, err := r.dir.ReadDir(len(items) - n)

		for _, dirent := range dirents {
			name := dirent.Name()
			if strings.HasPrefix(name, ".") {
				continue
			}

			info, err := dirent.Info()
			if err != nil {
				return n, err
			}

			tagsPath := filepath.Join(r.path, ".tags", name)
			tags, err := readTags(tagsPath)
			if err != nil {
				return n, err
			}

			items[n] = Info{
				Name:      path.Join(r.prefix, name),
				Size:      info.Size(),
				CreatedAt: info.ModTime(),
				Tags:      tags,
			}

			if query.MatchAll(&items[n], r.filters...) {
				n++
			}
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
