package sandbox

import (
	"context"
	"math"
	"path"
	"path/filepath"

	"golang.org/x/time/rate"

	"github.com/stealthrocket/wasi-go"
	wasisys "github.com/stealthrocket/wasi-go/systems/unix"
	sysunix "golang.org/x/sys/unix"
)

// FS is an interface used to represent a sandbox file system.
//
// The interface has a single method allowing the sandbox to open the root
// directory.
type FS interface {
	PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (File, wasi.Errno)
}

// ErrFS returns a FS value which always returns the given error.
func ErrFS(errno wasi.Errno) FS { return errFS(errno) }

type errFS wasi.Errno

func (err errFS) PathOpen(context.Context, wasi.LookupFlags, string, wasi.OpenFlags, wasi.Rights, wasi.Rights, wasi.FDFlags) (File, wasi.Errno) {
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

func (dir dirFS) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, filePath string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (File, wasi.Errno) {
	filePath = path.Join(string(dir), filePath)
	f, errno := wasisys.FD(sysunix.AT_FDCWD).PathOpen(ctx, lookupFlags, filePath, openFlags, rightsBase, rightsInheriting, fdFlags)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &dirFile{fd: f}, wasi.ESUCCESS
}

type dirFile struct {
	unimplementedSocketMethods
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

func (f *dirFile) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir File, newPath string) wasi.Errno {
	d, ok := newDir.(*dirFile)
	if !ok {
		return wasi.EXDEV
	}
	return f.fd.PathLink(ctx, flags, oldPath, d.fd, newPath)
}

func (f *dirFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (File, wasi.Errno) {
	fd, errno := f.fd.PathOpen(ctx, lookupFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
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

func (f *dirFile) PathRename(ctx context.Context, oldPath string, newDir File, newPath string) wasi.Errno {
	d, ok := newDir.(*dirFile)
	if !ok {
		return wasi.EXDEV
	}
	return f.fd.PathRename(ctx, oldPath, d.fd, newPath)
}

func (f *dirFile) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	return f.fd.PathSymlink(ctx, oldPath, newPath)
}

func (f *dirFile) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	return f.fd.PathUnlinkFile(ctx, path)
}

// ThrottleFS wraps the file system passed as argument to apply the rate limits
// r and w on read and write operations.
//
// The limits apply to all access to the underlying file system which may result
// in I/O operations.
//
// Passing a nil rate limiter to r or w disables rate limiting on the
// corresponding I/O operations.
func ThrottleFS(f FS, r, w *rate.Limiter) FS {
	return &throttleFS{base: f, rlim: r, wlim: w}
}

type throttleFS struct {
	base FS
	rlim *rate.Limiter
	wlim *rate.Limiter
}

func (fsys *throttleFS) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (File, wasi.Errno) {
	f, errno := fsys.base.PathOpen(ctx, lookupFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if canThrottle(errno) {
		fsys.throttlePathRead(ctx, path, 1)
	}
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &throttleFile{base: f, fsys: fsys}, wasi.ESUCCESS
}

func (fsys *throttleFS) throttlePathRead(ctx context.Context, path string, n int64) {
	fsys.throttleRead(ctx, int64(len(path))+alignIOPageSize(n))
}

func (fsys *throttleFS) throttlePathWrite(ctx context.Context, path string, n int64) {
	fsys.throttleRead(ctx, int64(len(path)))
	fsys.throttleWrite(ctx, n)
}

func (fsys *throttleFS) throttleRead(ctx context.Context, n int64) {
	throttle(ctx, fsys.rlim, alignIOPageSize(n))
}

func (fsys *throttleFS) throttleWrite(ctx context.Context, n int64) {
	throttle(ctx, fsys.wlim, alignIOPageSize(n))
}

func alignIOPageSize(n int64) int64 {
	const pageSize = 4096
	return ((n + pageSize - 1) / pageSize) * pageSize
}

func throttle(ctx context.Context, l *rate.Limiter, n int64) {
	if l == nil {
		return
	}
	for n > 0 {
		waitN := n
		if waitN > math.MaxInt32 {
			waitN = math.MaxInt32
		}
		if err := l.WaitN(ctx, int(waitN)); err != nil {
			panic(err)
		}
		n -= waitN
	}
}

type throttleFile struct {
	unimplementedSocketMethods
	base File
	fsys *throttleFS
}

func (f *throttleFile) FDPoll(ev wasi.EventType, ch chan<- struct{}) bool {
	return f.base.FDPoll(ev, ch)
}

func (f *throttleFile) FDClose(ctx context.Context) wasi.Errno {
	return f.base.FDClose(ctx)
}

func (f *throttleFile) FDAdvise(ctx context.Context, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	errno := f.base.FDAdvise(ctx, offset, length, advice)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, int64(length))
	}
	return errno
}

func (f *throttleFile) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	errno := f.base.FDAllocate(ctx, offset, length)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, int64(length))
	}
	return errno
}

func (f *throttleFile) FDDataSync(ctx context.Context) wasi.Errno {
	s, errno := f.base.FDFileStatGet(ctx)
	if errno != wasi.ESUCCESS {
		return errno
	}
	errno = f.base.FDDataSync(ctx)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, int64(s.Size))
	}
	return errno
}

func (f *throttleFile) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return f.base.FDStatSetFlags(ctx, flags)
}

func (f *throttleFile) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	stat, errno := f.base.FDFileStatGet(ctx)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, 1)
	}
	return stat, errno
}

func (f *throttleFile) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	errno := f.base.FDFileStatSetSize(ctx, size)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, int64(size))
	}
	return errno
}

func (f *throttleFile) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	errno := f.base.FDFileStatSetTimes(ctx, accessTime, modifyTime, flags)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, 1)
	}
	return errno
}

func (f *throttleFile) FDPread(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	n, errno := f.base.FDPread(ctx, iovecs, offset)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, sizeToInt64(n))
	}
	return n, errno
}

func (f *throttleFile) FDPwrite(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	n, errno := f.base.FDPwrite(ctx, iovecs, offset)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, sizeToInt64(n))
	}
	return n, errno
}

func (f *throttleFile) FDRead(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	n, errno := f.base.FDRead(ctx, iovecs)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, sizeToInt64(n))
	}
	return n, errno
}

func (f *throttleFile) FDWrite(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	n, errno := f.base.FDWrite(ctx, iovecs)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, sizeToInt64(n))
	}
	return n, errno
}

func (f *throttleFile) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	d, errno := f.base.FDOpenDir(ctx)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, 1)
	}
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &throttleDir{base: d, fsys: f.fsys}, wasi.ESUCCESS
}

func (f *throttleFile) FDSync(ctx context.Context) wasi.Errno {
	s, errno := f.base.FDFileStatGet(ctx)
	if errno != wasi.ESUCCESS {
		return errno
	}
	errno = f.base.FDSync(ctx)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, int64(s.Size))
	}
	return errno
}

func (f *throttleFile) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	offset, errno := f.base.FDSeek(ctx, delta, whence)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, 1)
	}
	return offset, errno
}

func (f *throttleFile) PathCreateDirectory(ctx context.Context, path string) wasi.Errno {
	errno := f.base.PathCreateDirectory(ctx, path)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, path, 1)
	}
	return errno
}

func (f *throttleFile) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	stat, errno := f.base.PathFileStatGet(ctx, flags, path)
	if canThrottle(errno) {
		f.fsys.throttlePathRead(ctx, path, 1)
	}
	return stat, errno
}

func (f *throttleFile) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	errno := f.base.PathFileStatSetTimes(ctx, lookupFlags, path, accessTime, modifyTime, fstFlags)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, path, 1)
	}
	return errno
}

func (f *throttleFile) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir File, newPath string) wasi.Errno {
	d, ok := newDir.(*throttleFile)
	if !ok {
		return wasi.EXDEV
	}
	errno := f.base.PathLink(ctx, flags, oldPath, d.base, newPath)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, oldPath, int64(len(newPath)))
	}
	return errno
}

func (f *throttleFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (File, wasi.Errno) {
	newFile, errno := f.base.PathOpen(ctx, lookupFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if canThrottle(errno) {
		f.fsys.throttlePathRead(ctx, path, 1)
	}
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &throttleFile{base: newFile, fsys: f.fsys}, wasi.ESUCCESS
}

func (f *throttleFile) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	n, errno := f.base.PathReadLink(ctx, path, buffer)
	if canThrottle(errno) {
		f.fsys.throttlePathRead(ctx, path, int64(n))
	}
	return n, errno
}

func (f *throttleFile) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	errno := f.base.PathRemoveDirectory(ctx, path)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, path, 1)
	}
	return errno
}

func (f *throttleFile) PathRename(ctx context.Context, oldPath string, newDir File, newPath string) wasi.Errno {
	d, ok := newDir.(*throttleFile)
	if !ok {
		return wasi.EXDEV
	}
	errno := f.base.PathRename(ctx, oldPath, d.base, newPath)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, oldPath, int64(len(newPath)))
	}
	return errno
}

func (f *throttleFile) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	errno := f.base.PathSymlink(ctx, oldPath, newPath)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, oldPath, int64(len(newPath)))
	}
	return errno
}

func (f *throttleFile) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	errno := f.base.PathUnlinkFile(ctx, path)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, path, 1)
	}
	return errno
}

// canThrottle returns true if the given errno code indicates that throttling
// can be applied to the operation that it was returned from.
//
// Throttling is always applied after performing the operation because we cannot
// know in advance whether the arguments passed to the method are valid; we have
// to first make the call and determine after the fact if the error returned by
// the method indicates that the operation was aborted due to having invalid
// arguments, or it was attempted and we need to take the I/O operation cost
// into account.
//
// The list of errors here may not be exhaustive; future maintainers may choose
// to add more. Keep in mind that we are better off apply throttling in excess
// than missing conditions where it should be applied because malcious guests
// could take advantage of error conditions that caused I/O utilization but were
// not accounted for.
func canThrottle(errno wasi.Errno) bool {
	return errno != wasi.EBADF && errno != wasi.EINVAL && errno != wasi.ENOTCAPABLE
}

func sizeToInt64(size wasi.Size) int64 {
	return int64(int32(size)) // for sign extension
}

type throttleDir struct {
	base wasi.Dir
	fsys *throttleFS
}

func (d *throttleDir) FDReadDir(ctx context.Context, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	n, errno := d.base.FDReadDir(ctx, entries, cookie, bufferSizeBytes)
	if canThrottle(errno) {
		size := int64(0)
		for _, entry := range entries[:n] {
			size += wasi.SizeOfDirent
			size += int64(len(entry.Name))
		}
		d.fsys.throttleRead(ctx, size)
	}
	return n, errno
}

func (d *throttleDir) FDCloseDir(ctx context.Context) wasi.Errno {
	return d.base.FDCloseDir(ctx)
}
