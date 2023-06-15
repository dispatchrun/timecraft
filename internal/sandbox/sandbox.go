package sandbox

import (
	"context"

	"github.com/stealthrocket/wasi-go"
)

// File is the interface implemented by all flavors of files that can be
// registered in a sandboxed System.
//
// The interface is an extension of wasi.File which adds socket methods and
// the ability to hook into the system's poller via two methods: Hook and Pool.
//
// Hook is called when the system needs to install a channel to be notified
// when an event triggers on the file.
//
// Poll is called to ask the file if the given event has triggered, and
// atomically clear that event's state if it did.
type File interface {
	wasi.File[File]
	Hook(ev wasi.EventType, ch chan<- struct{})
	Poll(ev wasi.EventType) bool
	SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno)
	SockBind(ctx context.Context, addr wasi.SocketAddress) wasi.Errno
	SockConnect(ctx context.Context, peer wasi.SocketAddress) wasi.Errno
	SockListen(ctx context.Context, backlog int) wasi.Errno
	SockRecv(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno)
	SockSend(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno)
	SockSendTo(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno)
	SockRecvFrom(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno)
	SockGetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno)
	SockSetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno
	SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno)
	SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno)
	SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno
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

func (defaultFile) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir File, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsDefault, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (File, wasi.Errno) {
	return nil, wasi.EBADF
}

func (defaultFile) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	return 0, wasi.EBADF
}

func (defaultFile) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathRename(ctx context.Context, oldPath string, newDir File, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

func (defaultFile) SockBind(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.ENOTSOCK
}

func (defaultFile) SockConnect(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.ENOTSOCK
}

func (defaultFile) SockListen(ctx context.Context, backlog int) wasi.Errno {
	return wasi.ENOTSOCK
}

func (defaultFile) SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (defaultFile) SockRecv(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	return 0, 0, wasi.ENOTSOCK
}

func (defaultFile) SockSend(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOTSOCK
}

func (defaultFile) SockSendTo(ctx context.Context, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOTSOCK
}

func (defaultFile) SockRecvFrom(ctx context.Context, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	return 0, 0, nil, wasi.ENOTSOCK
}

func (defaultFile) SockGetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (defaultFile) SockSetOpt(ctx context.Context, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	return wasi.ENOTSOCK
}

func (defaultFile) SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (defaultFile) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (defaultFile) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	return wasi.ENOTSOCK
}
