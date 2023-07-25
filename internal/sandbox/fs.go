package sandbox

import (
	"io/fs"
)

type FileMode = fs.FileMode

type FileInfo = fs.FileInfo

type FileSystem interface {
	Open(name string, flags int, mode FileMode) (File, error)
}

type File interface {
	Name() string

	Close() error

	Stat() (FileInfo, error)

	Open(name string, flags int, mode FileMode) (File, error)

	Read(data []byte) (int, error)

	Readv(iovs [][]byte) (int, error)

	Write(data []byte) (int, error)

	Writev(iovs [][]byte) (int, error)

	Pread(data []byte, off int64) (int, error)

	Preadv(iovs [][]byte, off int64) (int, error)

	Pwrite(data []byte, off int64) (int, error)

	Pwritev(iovs [][]byte, off int64) (int, error)

	Seek(offset int64, whence int) (int64, error)
}
