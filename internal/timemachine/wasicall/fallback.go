package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
)

// FallbackSystem wraps two System instances. If the first returns ENOSYS
// for a particular system call, the call is forwarded to the second instance.
//
// Preopen and Register calls are forwarded to the primary system only.
// Close however closes both systems.
type FallbackSystem struct {
	System

	secondary System
}

var _ System = (*FallbackSystem)(nil)
var _ SocketsExtension = (*FallbackSystem)(nil)

// NewFallbackSystem creates a FallbackSystem.
func NewFallbackSystem(primary, secondary System) *FallbackSystem {
	return &FallbackSystem{
		System:    primary,
		secondary: secondary,
	}
}

func (f *FallbackSystem) ArgsSizesGet(ctx context.Context) (int, int, Errno) {
	argCount, stringBytes, errno := f.System.ArgsSizesGet(ctx)
	if errno == ENOSYS {
		return f.secondary.ArgsSizesGet(ctx)
	}
	return argCount, stringBytes, errno
}

func (f *FallbackSystem) ArgsGet(ctx context.Context) ([]string, Errno) {
	args, errno := f.System.ArgsGet(ctx)
	if errno == ENOSYS {
		return f.secondary.ArgsGet(ctx)
	}
	return args, errno
}

func (f *FallbackSystem) EnvironSizesGet(ctx context.Context) (int, int, Errno) {
	envCount, stringBytes, errno := f.System.EnvironSizesGet(ctx)
	if errno == ENOSYS {
		return f.secondary.EnvironSizesGet(ctx)
	}
	return envCount, stringBytes, errno
}

func (f *FallbackSystem) EnvironGet(ctx context.Context) ([]string, Errno) {
	env, errno := f.System.EnvironGet(ctx)
	if errno == ENOSYS {
		return f.secondary.EnvironGet(ctx)
	}
	return env, errno
}

func (f *FallbackSystem) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	precision, errno := f.System.ClockResGet(ctx, id)
	if errno == ENOSYS {
		return f.secondary.ClockResGet(ctx, id)
	}
	return precision, errno
}

func (f *FallbackSystem) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	timestamp, errno := f.System.ClockTimeGet(ctx, id, precision)
	if errno == ENOSYS {
		return f.secondary.ClockTimeGet(ctx, id, precision)
	}
	return timestamp, errno
}

func (f *FallbackSystem) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	errno := f.System.FDAdvise(ctx, fd, offset, length, advice)
	if errno == ENOSYS {
		return f.secondary.FDAdvise(ctx, fd, offset, length, advice)
	}
	return errno
}

func (f *FallbackSystem) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	errno := f.System.FDAllocate(ctx, fd, offset, length)
	if errno == ENOSYS {
		return f.secondary.FDAllocate(ctx, fd, offset, length)
	}
	return errno
}

func (f *FallbackSystem) FDClose(ctx context.Context, fd FD) Errno {
	errno := f.System.FDClose(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDClose(ctx, fd)
	}
	return errno
}

func (f *FallbackSystem) FDDataSync(ctx context.Context, fd FD) Errno {
	errno := f.System.FDDataSync(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDDataSync(ctx, fd)
	}
	return errno
}

func (f *FallbackSystem) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	stat, errno := f.System.FDStatGet(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDStatGet(ctx, fd)
	}
	return stat, errno
}

func (f *FallbackSystem) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	errno := f.System.FDStatSetFlags(ctx, fd, flags)
	if errno == ENOSYS {
		return f.secondary.FDStatSetFlags(ctx, fd, flags)
	}
	return errno
}

func (f *FallbackSystem) FDStatSetRights(ctx context.Context, fd FD, rightsBase, rightsInheriting Rights) Errno {
	errno := f.System.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	if errno == ENOSYS {
		return f.secondary.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	}
	return errno
}

func (f *FallbackSystem) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	stat, errno := f.System.FDFileStatGet(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDFileStatGet(ctx, fd)
	}
	return stat, errno
}

func (f *FallbackSystem) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	errno := f.System.FDFileStatSetSize(ctx, fd, size)
	if errno == ENOSYS {
		return f.secondary.FDFileStatSetSize(ctx, fd, size)
	}
	return errno
}

func (f *FallbackSystem) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := f.System.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	if errno == ENOSYS {
		return f.secondary.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	}
	return errno
}

func (f *FallbackSystem) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	size, errno := f.System.FDPread(ctx, fd, iovecs, offset)
	if errno == ENOSYS {
		return f.secondary.FDPread(ctx, fd, iovecs, offset)
	}
	return size, errno
}

func (f *FallbackSystem) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	stat, errno := f.System.FDPreStatGet(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDPreStatGet(ctx, fd)
	}
	return stat, errno
}

func (f *FallbackSystem) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	name, errno := f.System.FDPreStatDirName(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDPreStatDirName(ctx, fd)
	}
	return name, errno
}

func (f *FallbackSystem) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	size, errno := f.System.FDPwrite(ctx, fd, iovecs, offset)
	if errno == ENOSYS {
		return f.secondary.FDPwrite(ctx, fd, iovecs, offset)
	}
	return size, errno
}

func (f *FallbackSystem) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	size, errno := f.System.FDRead(ctx, fd, iovecs)
	if errno == ENOSYS {
		return f.secondary.FDRead(ctx, fd, iovecs)
	}
	return size, errno
}

func (f *FallbackSystem) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	count, errno := f.System.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	if errno == ENOSYS {
		return f.secondary.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	}
	return count, errno
}

func (f *FallbackSystem) FDRenumber(ctx context.Context, from, to FD) Errno {
	errno := f.System.FDRenumber(ctx, from, to)
	if errno == ENOSYS {
		return f.secondary.FDRenumber(ctx, from, to)
	}
	return errno
}

func (f *FallbackSystem) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	size, errno := f.System.FDSeek(ctx, fd, offset, whence)
	if errno == ENOSYS {
		return f.secondary.FDSeek(ctx, fd, offset, whence)
	}
	return size, errno
}

func (f *FallbackSystem) FDSync(ctx context.Context, fd FD) Errno {
	errno := f.System.FDSync(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDSync(ctx, fd)
	}
	return errno
}

func (f *FallbackSystem) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	size, errno := f.System.FDTell(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDTell(ctx, fd)
	}
	return size, errno
}

func (f *FallbackSystem) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	size, errno := f.System.FDWrite(ctx, fd, iovecs)
	if errno == ENOSYS {
		return f.secondary.FDWrite(ctx, fd, iovecs)
	}
	return size, errno
}

func (f *FallbackSystem) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := f.System.PathCreateDirectory(ctx, fd, path)
	if errno == ENOSYS {
		return f.secondary.PathCreateDirectory(ctx, fd, path)
	}
	return errno
}

func (f *FallbackSystem) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	stat, errno := f.System.PathFileStatGet(ctx, fd, lookupFlags, path)
	if errno == ENOSYS {
		return f.secondary.PathFileStatGet(ctx, fd, lookupFlags, path)
	}
	return stat, errno
}

func (f *FallbackSystem) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := f.System.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	if errno == ENOSYS {
		return f.secondary.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	}
	return errno
}

func (f *FallbackSystem) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	errno := f.System.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	if errno == ENOSYS {
		return f.secondary.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	}
	return errno
}

func (f *FallbackSystem) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	newfd, errno := f.System.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if errno == ENOSYS {
		return f.secondary.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	}
	return newfd, errno
}

func (f *FallbackSystem) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) (int, Errno) {
	n, errno := f.System.PathReadLink(ctx, fd, path, buffer)
	if errno == ENOSYS {
		return f.secondary.PathReadLink(ctx, fd, path, buffer)
	}
	return n, errno
}

func (f *FallbackSystem) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := f.System.PathRemoveDirectory(ctx, fd, path)
	if errno == ENOSYS {
		return f.secondary.PathRemoveDirectory(ctx, fd, path)
	}
	return errno
}

func (f *FallbackSystem) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	errno := f.System.PathRename(ctx, fd, oldPath, newFD, newPath)
	if errno == ENOSYS {
		return f.secondary.PathRename(ctx, fd, oldPath, newFD, newPath)
	}
	return errno
}

func (f *FallbackSystem) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	errno := f.System.PathSymlink(ctx, oldPath, fd, newPath)
	if errno == ENOSYS {
		return f.secondary.PathSymlink(ctx, oldPath, fd, newPath)
	}
	return errno
}

func (f *FallbackSystem) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	errno := f.System.PathUnlinkFile(ctx, fd, path)
	if errno == ENOSYS {
		return f.secondary.PathUnlinkFile(ctx, fd, path)
	}
	return errno
}

func (f *FallbackSystem) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	count, errno := f.System.PollOneOff(ctx, subscriptions, events)
	if errno == ENOSYS {
		return f.secondary.PollOneOff(ctx, subscriptions, events)
	}
	return count, errno
}

func (f *FallbackSystem) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	errno := f.System.ProcExit(ctx, exitCode)
	if errno == ENOSYS {
		return f.secondary.ProcExit(ctx, exitCode)
	}
	return errno
}

func (f *FallbackSystem) ProcRaise(ctx context.Context, signal Signal) Errno {
	errno := f.System.ProcRaise(ctx, signal)
	if errno == ENOSYS {
		return f.secondary.ProcRaise(ctx, signal)
	}
	return errno
}

func (f *FallbackSystem) SchedYield(ctx context.Context) Errno {
	errno := f.System.SchedYield(ctx)
	if errno == ENOSYS {
		return f.secondary.SchedYield(ctx)
	}
	return errno
}

func (f *FallbackSystem) RandomGet(ctx context.Context, b []byte) Errno {
	errno := f.System.RandomGet(ctx, b)
	if errno == ENOSYS {
		return f.secondary.RandomGet(ctx, b)
	}
	return errno
}

func (f *FallbackSystem) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, SocketAddress, SocketAddress, Errno) {
	newfd, peer, addr, errno := f.System.SockAccept(ctx, fd, flags)
	if errno == ENOSYS {
		return f.secondary.SockAccept(ctx, fd, flags)
	}
	return newfd, peer, addr, errno
}

func (f *FallbackSystem) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, Errno) {
	size, oflags, errno := f.System.SockRecv(ctx, fd, iovecs, iflags)
	if errno == ENOSYS {
		return f.secondary.SockRecv(ctx, fd, iovecs, iflags)
	}
	return size, oflags, errno
}

func (f *FallbackSystem) SockSend(ctx context.Context, fd FD, iovecs []IOVec, flags SIFlags) (Size, Errno) {
	size, errno := f.System.SockSend(ctx, fd, iovecs, flags)
	if errno == ENOSYS {
		return f.secondary.SockSend(ctx, fd, iovecs, flags)
	}
	return size, errno
}

func (f *FallbackSystem) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	errno := f.System.SockShutdown(ctx, fd, flags)
	if errno == ENOSYS {
		return f.secondary.SockShutdown(ctx, fd, flags)
	}
	return errno
}

func (f *FallbackSystem) SockOpen(ctx context.Context, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights) (newfd FD, errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	newfd, errno = se.SockOpen(ctx, family, socketType, protocol, rightsBase, rightsInheriting)
	if errno == ENOSYS {
		goto fallback
	}
	return newfd, errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return -1, ENOSYS
	}
	return se.SockOpen(ctx, family, socketType, protocol, rightsBase, rightsInheriting)
}

func (f *FallbackSystem) SockBind(ctx context.Context, fd FD, bind SocketAddress) (addr SocketAddress, errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	addr, errno = se.SockBind(ctx, fd, bind)
	if errno == ENOSYS {
		goto fallback
	}
	return addr, errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	return se.SockBind(ctx, fd, bind)
}

func (f *FallbackSystem) SockConnect(ctx context.Context, fd FD, peer SocketAddress) (addr SocketAddress, errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	addr, errno = se.SockConnect(ctx, fd, peer)
	if errno == ENOSYS {
		goto fallback
	}
	return addr, errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	return se.SockConnect(ctx, fd, peer)
}

func (f *FallbackSystem) SockListen(ctx context.Context, fd FD, backlog int) (errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	errno = se.SockListen(ctx, fd, backlog)
	if errno == ENOSYS {
		goto fallback
	}
	return errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	return se.SockListen(ctx, fd, backlog)
}

func (f *FallbackSystem) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, flags SIFlags, addr SocketAddress) (size Size, errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	size, errno = se.SockSendTo(ctx, fd, iovecs, flags, addr)
	if errno == ENOSYS {
		goto fallback
	}
	return size, errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return 0, ENOSYS
	}
	return se.SockSendTo(ctx, fd, iovecs, flags, addr)
}

func (f *FallbackSystem) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, flags RIFlags) (size Size, oflags ROFlags, addr SocketAddress, errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	size, oflags, addr, errno = se.SockRecvFrom(ctx, fd, iovecs, flags)
	if errno == ENOSYS {
		goto fallback
	}
	return size, oflags, addr, errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return 0, 0, nil, ENOSYS
	}
	return se.SockRecvFrom(ctx, fd, iovecs, flags)
}

func (f *FallbackSystem) SockGetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (value int, errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	value, errno = se.SockGetOptInt(ctx, fd, level, option)
	if errno == ENOSYS {
		goto fallback
	}
	return value, errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return 0, ENOSYS
	}
	return se.SockGetOptInt(ctx, fd, level, option)
}

func (f *FallbackSystem) SockSetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value int) (errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	errno = se.SockSetOptInt(ctx, fd, level, option, value)
	if errno == ENOSYS {
		goto fallback
	}
	return errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	return se.SockSetOptInt(ctx, fd, level, option, value)
}

func (f *FallbackSystem) SockLocalAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	addr, errno = se.SockLocalAddress(ctx, fd)
	if errno == ENOSYS {
		goto fallback
	}
	return addr, errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	return se.SockLocalAddress(ctx, fd)
}

func (f *FallbackSystem) SockRemoteAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	se, ok := f.System.(SocketsExtension)
	if !ok {
		goto fallback
	}
	addr, errno = se.SockRemoteAddress(ctx, fd)
	if errno == ENOSYS {
		goto fallback
	}
	return addr, errno
fallback:
	se, ok = f.secondary.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	return se.SockRemoteAddress(ctx, fd)
}

func (f *FallbackSystem) Close(ctx context.Context) error {
	err := f.System.Close(ctx)
	err2 := f.secondary.Close(ctx)
	if err == nil {
		err = err2
	}
	return err
}
