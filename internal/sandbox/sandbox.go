package sandbox

import (
	"context"

	"github.com/stealthrocket/wasi-go"
)

// File is the interface implemented by all flavors of files that can be
// registered in a sandboxed System.
type File interface {
	wasi.File[File]
	Unwrap() File
	FDPoll(ev wasi.EventType, ch chan<- struct{}) bool
	SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno)
	SockBind(ctx context.Context, addr wasi.SocketAddress) wasi.Errno
	SockConnect(ctx context.Context, peer wasi.SocketAddress) wasi.Errno
	SockListen(ctx context.Context, backlog int) wasi.Errno
	SockRecv(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno)
	SockSend(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno)
	SockSendTo(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno)
	SockRecvFrom(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno)
	SockGetOpt(ctx context.Context, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno)
	SockSetOpt(ctx context.Context, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno
	SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno)
	SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno)
	SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno
}

func unwrap(f File) File {
	for {
		if u := f.Unwrap(); u != nil {
			f = u
		} else {
			return f
		}
	}
}

// unimplementedFileSystemMethods declares all the methods of the File interface
// that are not supported by implementations which are not files or directories.
type unimplementedFileSystemMethods struct{}

func (unimplementedFileSystemMethods) FDAdvise(ctx context.Context, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) FDDataSync(ctx context.Context) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) FDPread(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return 0, wasi.EBADF
}

func (unimplementedFileSystemMethods) FDPwrite(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return 0, wasi.EBADF
}

func (unimplementedFileSystemMethods) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	return nil, wasi.EBADF
}

func (unimplementedFileSystemMethods) FDSync(ctx context.Context) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	return 0, wasi.EBADF
}

func (unimplementedFileSystemMethods) PathCreateDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return wasi.FileStat{}, wasi.EBADF
}

func (unimplementedFileSystemMethods) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir File, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsDefault, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (File, wasi.Errno) {
	return nil, wasi.EBADF
}

func (unimplementedFileSystemMethods) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	return 0, wasi.EBADF
}

func (unimplementedFileSystemMethods) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) PathRename(ctx context.Context, oldPath string, newDir File, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileSystemMethods) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

// unimplementedSocketMethods is useful to declare all file methods as not implemented by
// embedding the type.
//
// Only methods that are not valid to call or files and directories are declared.
type unimplementedSocketMethods struct{}

func (unimplementedSocketMethods) SockBind(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockConnect(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockListen(ctx context.Context, backlog int) wasi.Errno {
	return wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockRecv(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	return 0, 0, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockSend(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockSendTo(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockRecvFrom(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	return 0, 0, nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockGetOpt(ctx context.Context, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockSetOpt(ctx context.Context, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	return wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	return wasi.ENOTSOCK
}
