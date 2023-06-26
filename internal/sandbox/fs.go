package sandbox

import (
	"context"
	"path"
	"path/filepath"

	"github.com/stealthrocket/wasi-go"
	wasisys "github.com/stealthrocket/wasi-go/systems/unix"
	sysunix "golang.org/x/sys/unix"
)

// FS is an interface used to represent a sandbox file system.
//
// The interface has a single method allowing the sandbox to open the root
// directory.
type FS interface {
	PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsDefault, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (AnyFile, wasi.Errno)
}

// ErrFS returns a FS value which always returns the given error.
func ErrFS(errno wasi.Errno) FS { return errFS(errno) }

type errFS wasi.Errno

func (err errFS) PathOpen(context.Context, wasi.LookupFlags, string, wasi.OpenFlags, wasi.Rights, wasi.Rights, wasi.FDFlags) (AnyFile, wasi.Errno) {
	return nil, wasi.Errno(err)
}

// DirFS returns a FS value which opens files from the given file system path.
//
// The path is resolved to an absolute path in order to guarantee that the FS is
// not dependent on the current working directory.
func DirFS(path string) FS {
	path, err := filepath.Abs(path)
	if err != nil {
		return ErrFS(wasi.MakeErrno(err))
	}
	return dirFS(filepath.ToSlash(path))
}

type dirFS string

func (dir dirFS) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, filePath string, openFlags wasi.OpenFlags, rightsDefault, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (AnyFile, wasi.Errno) {
	filePath = path.Join(string(dir), filePath)
	f, errno := wasisys.FD(sysunix.AT_FDCWD).PathOpen(ctx, lookupFlags, filePath, openFlags, rightsDefault, rightsInheriting, fdFlags)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &dirFile{fd: f}, wasi.ESUCCESS
}

type dirFile struct {
	defaultFile
	fd wasisys.FD
}

func (f *dirFile) FDPoll(ev wasi.EventType, ch chan<- struct{}) bool {
	return true
}

func (f *dirFile) FDClose(ctx context.Context) wasi.Errno {
	return f.fd.FDClose(ctx)
}

func (f *dirFile) FDAdvise(ctx context.Context, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return f.fd.FDAdvise(ctx, offset, length, advice)
}

func (f *dirFile) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	return f.fd.FDAllocate(ctx, offset, length)
}

func (f *dirFile) FDDataSync(ctx context.Context) wasi.Errno {
	return f.fd.FDDataSync(ctx)
}

func (f *dirFile) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return f.fd.FDStatSetFlags(ctx, flags)
}

func (f *dirFile) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	return f.fd.FDFileStatGet(ctx)
}

func (f *dirFile) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	return f.fd.FDFileStatSetSize(ctx, size)
}

func (f *dirFile) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return f.fd.FDFileStatSetTimes(ctx, accessTime, modifyTime, flags)
}

func (f *dirFile) FDPread(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return f.fd.FDPread(ctx, iovecs, offset)
}

func (f *dirFile) FDPwrite(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return f.fd.FDPwrite(ctx, iovecs, offset)
}

func (f *dirFile) FDRead(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return f.fd.FDRead(ctx, iovecs)
}

func (f *dirFile) FDWrite(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return f.fd.FDWrite(ctx, iovecs)
}

func (f *dirFile) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	return f.fd.FDOpenDir(ctx)
}

func (f *dirFile) FDSync(ctx context.Context) wasi.Errno {
	return f.fd.FDSync(ctx)
}

func (f *dirFile) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	return f.fd.FDSeek(ctx, delta, whence)
}

func (f *dirFile) PathCreateDirectory(ctx context.Context, path string) wasi.Errno {
	return f.fd.PathCreateDirectory(ctx, path)
}

func (f *dirFile) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return f.fd.PathFileStatGet(ctx, flags, path)
}

func (f *dirFile) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	return f.fd.PathFileStatSetTimes(ctx, lookupFlags, path, accessTime, modifyTime, fstFlags)
}

func (f *dirFile) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir AnyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*dirFile)
	if !ok {
		return wasi.ENOTDIR
	}
	return f.fd.PathLink(ctx, flags, oldPath, d.fd, newPath)
}

func (f *dirFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsDefault, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (AnyFile, wasi.Errno) {
	fd, errno := f.fd.PathOpen(ctx, lookupFlags, path, openFlags, rightsDefault, rightsInheriting, fdFlags)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &dirFile{fd: fd}, wasi.ESUCCESS
}

func (f *dirFile) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	return f.fd.PathReadLink(ctx, path, buffer)
}

func (f *dirFile) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	return f.fd.PathRemoveDirectory(ctx, path)
}

func (f *dirFile) PathRename(ctx context.Context, oldPath string, newDir AnyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*dirFile)
	if !ok {
		return wasi.ENOTDIR
	}
	return f.fd.PathRename(ctx, oldPath, d.fd, newPath)
}

func (f *dirFile) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	return f.fd.PathSymlink(ctx, oldPath, newPath)
}

func (f *dirFile) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	return f.fd.PathUnlinkFile(ctx, path)
}
