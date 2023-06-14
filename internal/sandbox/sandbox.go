package sandbox

import (
	"context"

	"github.com/stealthrocket/wasi-go"
)

type anyFile interface {
	wasi.File[anyFile]
	hook(ev wasi.EventType, ch chan<- struct{})
	poll(ev wasi.EventType) bool
}

// defaultFile is useful to declare all file methods as not implemented by
// embedding the type.
//
// Note that FDClose isn't declared because it is critical to the lifecycle of
// all files, so we want to leverage the compiler to make sure it always exists.
type defaultFile struct{}

func (defaultFile) FDAdvise(ctx context.Context, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) FDDataSync(ctx context.Context) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	return wasi.FileStat{}, wasi.EBADF
}

func (defaultFile) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) FDPread(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return 0, wasi.EBADF
}

func (defaultFile) FDPwrite(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return 0, wasi.EBADF
}

func (defaultFile) FDRead(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return 0, wasi.EBADF
}

func (defaultFile) FDWrite(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return 0, wasi.EBADF
}

func (defaultFile) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	return nil, wasi.EBADF
}

func (defaultFile) FDSync(ctx context.Context) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	return 0, wasi.EBADF
}

func (defaultFile) PathCreateDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return wasi.FileStat{}, wasi.EBADF
}

func (defaultFile) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsDefault, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	return nil, wasi.EBADF
}

func (defaultFile) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	return 0, wasi.EBADF
}

func (defaultFile) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathRename(ctx context.Context, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}
