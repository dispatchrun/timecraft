package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
)

// NewFallbackSystem creates a wasi.System that wraps two other wasi.System
// instances, a primary and a secondary. Calls are first forwarded to the
// primary system, and then forwarded to the secondary system when the primary
// returns wasi.ENOSYS.
func NewFallbackSystem(primary, secondary System) System {
	return &fallbackSystem{primary, secondary}
}

type fallbackSystem struct {
	primary   System
	secondary System
}

func (f *fallbackSystem) ArgsSizesGet(ctx context.Context) (int, int, Errno) {
	argCount, stringBytes, errno := f.primary.ArgsSizesGet(ctx)
	if errno == ENOSYS {
		return f.secondary.ArgsSizesGet(ctx)
	}
	return argCount, stringBytes, errno
}

func (f *fallbackSystem) ArgsGet(ctx context.Context) ([]string, Errno) {
	args, errno := f.primary.ArgsGet(ctx)
	if errno == ENOSYS {
		return f.secondary.ArgsGet(ctx)
	}
	return args, errno
}

func (f *fallbackSystem) EnvironSizesGet(ctx context.Context) (int, int, Errno) {
	envCount, stringBytes, errno := f.primary.EnvironSizesGet(ctx)
	if errno == ENOSYS {
		return f.secondary.EnvironSizesGet(ctx)
	}
	return envCount, stringBytes, errno
}

func (f *fallbackSystem) EnvironGet(ctx context.Context) ([]string, Errno) {
	env, errno := f.primary.EnvironGet(ctx)
	if errno == ENOSYS {
		return f.secondary.EnvironGet(ctx)
	}
	return env, errno
}

func (f *fallbackSystem) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	precision, errno := f.primary.ClockResGet(ctx, id)
	if errno == ENOSYS {
		return f.secondary.ClockResGet(ctx, id)
	}
	return precision, errno
}

func (f *fallbackSystem) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	timestamp, errno := f.primary.ClockTimeGet(ctx, id, precision)
	if errno == ENOSYS {
		return f.secondary.ClockTimeGet(ctx, id, precision)
	}
	return timestamp, errno
}

func (f *fallbackSystem) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	errno := f.primary.FDAdvise(ctx, fd, offset, length, advice)
	if errno == ENOSYS {
		return f.secondary.FDAdvise(ctx, fd, offset, length, advice)
	}
	return errno
}

func (f *fallbackSystem) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	errno := f.primary.FDAllocate(ctx, fd, offset, length)
	if errno == ENOSYS {
		return f.secondary.FDAllocate(ctx, fd, offset, length)
	}
	return errno
}

func (f *fallbackSystem) FDClose(ctx context.Context, fd FD) Errno {
	errno := f.primary.FDClose(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDClose(ctx, fd)
	}
	return errno
}

func (f *fallbackSystem) FDDataSync(ctx context.Context, fd FD) Errno {
	errno := f.primary.FDDataSync(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDDataSync(ctx, fd)
	}
	return errno
}

func (f *fallbackSystem) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	stat, errno := f.primary.FDStatGet(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDStatGet(ctx, fd)
	}
	return stat, errno
}

func (f *fallbackSystem) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	errno := f.primary.FDStatSetFlags(ctx, fd, flags)
	if errno == ENOSYS {
		return f.secondary.FDStatSetFlags(ctx, fd, flags)
	}
	return errno
}

func (f *fallbackSystem) FDStatSetRights(ctx context.Context, fd FD, rightsBase, rightsInheriting Rights) Errno {
	errno := f.primary.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	if errno == ENOSYS {
		return f.secondary.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	}
	return errno
}

func (f *fallbackSystem) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	stat, errno := f.primary.FDFileStatGet(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDFileStatGet(ctx, fd)
	}
	return stat, errno
}

func (f *fallbackSystem) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	errno := f.primary.FDFileStatSetSize(ctx, fd, size)
	if errno == ENOSYS {
		return f.secondary.FDFileStatSetSize(ctx, fd, size)
	}
	return errno
}

func (f *fallbackSystem) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := f.primary.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	if errno == ENOSYS {
		return f.secondary.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	}
	return errno
}

func (f *fallbackSystem) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	size, errno := f.primary.FDPread(ctx, fd, iovecs, offset)
	if errno == ENOSYS {
		return f.secondary.FDPread(ctx, fd, iovecs, offset)
	}
	return size, errno
}

func (f *fallbackSystem) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	stat, errno := f.primary.FDPreStatGet(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDPreStatGet(ctx, fd)
	}
	return stat, errno
}

func (f *fallbackSystem) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	name, errno := f.primary.FDPreStatDirName(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDPreStatDirName(ctx, fd)
	}
	return name, errno
}

func (f *fallbackSystem) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	size, errno := f.primary.FDPwrite(ctx, fd, iovecs, offset)
	if errno == ENOSYS {
		return f.secondary.FDPwrite(ctx, fd, iovecs, offset)
	}
	return size, errno
}

func (f *fallbackSystem) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	size, errno := f.primary.FDRead(ctx, fd, iovecs)
	if errno == ENOSYS {
		return f.secondary.FDRead(ctx, fd, iovecs)
	}
	return size, errno
}

func (f *fallbackSystem) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	count, errno := f.primary.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	if errno == ENOSYS {
		return f.secondary.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	}
	return count, errno
}

func (f *fallbackSystem) FDRenumber(ctx context.Context, from, to FD) Errno {
	errno := f.primary.FDRenumber(ctx, from, to)
	if errno == ENOSYS {
		return f.secondary.FDRenumber(ctx, from, to)
	}
	return errno
}

func (f *fallbackSystem) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	size, errno := f.primary.FDSeek(ctx, fd, offset, whence)
	if errno == ENOSYS {
		return f.secondary.FDSeek(ctx, fd, offset, whence)
	}
	return size, errno
}

func (f *fallbackSystem) FDSync(ctx context.Context, fd FD) Errno {
	errno := f.primary.FDSync(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDSync(ctx, fd)
	}
	return errno
}

func (f *fallbackSystem) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	size, errno := f.primary.FDTell(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.FDTell(ctx, fd)
	}
	return size, errno
}

func (f *fallbackSystem) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	size, errno := f.primary.FDWrite(ctx, fd, iovecs)
	if errno == ENOSYS {
		return f.secondary.FDWrite(ctx, fd, iovecs)
	}
	return size, errno
}

func (f *fallbackSystem) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := f.primary.PathCreateDirectory(ctx, fd, path)
	if errno == ENOSYS {
		return f.secondary.PathCreateDirectory(ctx, fd, path)
	}
	return errno
}

func (f *fallbackSystem) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	stat, errno := f.primary.PathFileStatGet(ctx, fd, lookupFlags, path)
	if errno == ENOSYS {
		return f.secondary.PathFileStatGet(ctx, fd, lookupFlags, path)
	}
	return stat, errno
}

func (f *fallbackSystem) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := f.primary.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	if errno == ENOSYS {
		return f.secondary.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	}
	return errno
}

func (f *fallbackSystem) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	errno := f.primary.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	if errno == ENOSYS {
		return f.secondary.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	}
	return errno
}

func (f *fallbackSystem) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	newfd, errno := f.primary.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if errno == ENOSYS {
		return f.secondary.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	}
	return newfd, errno
}

func (f *fallbackSystem) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) (int, Errno) {
	n, errno := f.primary.PathReadLink(ctx, fd, path, buffer)
	if errno == ENOSYS {
		return f.secondary.PathReadLink(ctx, fd, path, buffer)
	}
	return n, errno
}

func (f *fallbackSystem) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := f.primary.PathRemoveDirectory(ctx, fd, path)
	if errno == ENOSYS {
		return f.secondary.PathRemoveDirectory(ctx, fd, path)
	}
	return errno
}

func (f *fallbackSystem) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	errno := f.primary.PathRename(ctx, fd, oldPath, newFD, newPath)
	if errno == ENOSYS {
		return f.secondary.PathRename(ctx, fd, oldPath, newFD, newPath)
	}
	return errno
}

func (f *fallbackSystem) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	errno := f.primary.PathSymlink(ctx, oldPath, fd, newPath)
	if errno == ENOSYS {
		return f.secondary.PathSymlink(ctx, oldPath, fd, newPath)
	}
	return errno
}

func (f *fallbackSystem) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	errno := f.primary.PathUnlinkFile(ctx, fd, path)
	if errno == ENOSYS {
		return f.secondary.PathUnlinkFile(ctx, fd, path)
	}
	return errno
}

func (f *fallbackSystem) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	count, errno := f.primary.PollOneOff(ctx, subscriptions, events)
	if errno == ENOSYS {
		return f.secondary.PollOneOff(ctx, subscriptions, events)
	}
	return count, errno
}

func (f *fallbackSystem) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	errno := f.primary.ProcExit(ctx, exitCode)
	if errno == ENOSYS {
		return f.secondary.ProcExit(ctx, exitCode)
	}
	return errno
}

func (f *fallbackSystem) ProcRaise(ctx context.Context, signal Signal) Errno {
	errno := f.primary.ProcRaise(ctx, signal)
	if errno == ENOSYS {
		return f.secondary.ProcRaise(ctx, signal)
	}
	return errno
}

func (f *fallbackSystem) SchedYield(ctx context.Context) Errno {
	errno := f.primary.SchedYield(ctx)
	if errno == ENOSYS {
		return f.secondary.SchedYield(ctx)
	}
	return errno
}

func (f *fallbackSystem) RandomGet(ctx context.Context, b []byte) Errno {
	errno := f.primary.RandomGet(ctx, b)
	if errno == ENOSYS {
		return f.secondary.RandomGet(ctx, b)
	}
	return errno
}

func (f *fallbackSystem) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, SocketAddress, SocketAddress, Errno) {
	newfd, peer, addr, errno := f.primary.SockAccept(ctx, fd, flags)
	if errno == ENOSYS {
		return f.secondary.SockAccept(ctx, fd, flags)
	}
	return newfd, peer, addr, errno
}

func (f *fallbackSystem) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, Errno) {
	size, oflags, errno := f.primary.SockRecv(ctx, fd, iovecs, iflags)
	if errno == ENOSYS {
		return f.secondary.SockRecv(ctx, fd, iovecs, iflags)
	}
	return size, oflags, errno
}

func (f *fallbackSystem) SockSend(ctx context.Context, fd FD, iovecs []IOVec, flags SIFlags) (Size, Errno) {
	size, errno := f.primary.SockSend(ctx, fd, iovecs, flags)
	if errno == ENOSYS {
		return f.secondary.SockSend(ctx, fd, iovecs, flags)
	}
	return size, errno
}

func (f *fallbackSystem) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	errno := f.primary.SockShutdown(ctx, fd, flags)
	if errno == ENOSYS {
		return f.secondary.SockShutdown(ctx, fd, flags)
	}
	return errno
}

func (f *fallbackSystem) SockOpen(ctx context.Context, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights) (newfd FD, errno Errno) {
	newfd, errno = f.primary.SockOpen(ctx, family, socketType, protocol, rightsBase, rightsInheriting)
	if errno == ENOSYS {
		return f.secondary.SockOpen(ctx, family, socketType, protocol, rightsBase, rightsInheriting)
	}
	return newfd, errno
}

func (f *fallbackSystem) SockBind(ctx context.Context, fd FD, bind SocketAddress) (addr SocketAddress, errno Errno) {
	addr, errno = f.primary.SockBind(ctx, fd, bind)
	if errno == ENOSYS {
		return f.secondary.SockBind(ctx, fd, bind)
	}
	return addr, errno
}

func (f *fallbackSystem) SockConnect(ctx context.Context, fd FD, peer SocketAddress) (addr SocketAddress, errno Errno) {
	addr, errno = f.primary.SockConnect(ctx, fd, peer)
	if errno == ENOSYS {
		return f.secondary.SockConnect(ctx, fd, peer)
	}
	return addr, errno
}

func (f *fallbackSystem) SockListen(ctx context.Context, fd FD, backlog int) (errno Errno) {
	errno = f.primary.SockListen(ctx, fd, backlog)
	if errno == ENOSYS {
		return f.secondary.SockListen(ctx, fd, backlog)
	}
	return errno
}

func (f *fallbackSystem) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, flags SIFlags, addr SocketAddress) (size Size, errno Errno) {
	size, errno = f.primary.SockSendTo(ctx, fd, iovecs, flags, addr)
	if errno == ENOSYS {
		return f.secondary.SockSendTo(ctx, fd, iovecs, flags, addr)
	}
	return size, errno
}

func (f *fallbackSystem) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, flags RIFlags) (size Size, oflags ROFlags, addr SocketAddress, errno Errno) {
	size, oflags, addr, errno = f.primary.SockRecvFrom(ctx, fd, iovecs, flags)
	if errno == ENOSYS {
		return f.secondary.SockRecvFrom(ctx, fd, iovecs, flags)
	}
	return size, oflags, addr, errno
}

func (f *fallbackSystem) SockGetOpt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (value SocketOptionValue, errno Errno) {
	value, errno = f.primary.SockGetOpt(ctx, fd, level, option)
	if errno == ENOSYS {
		return f.secondary.SockGetOpt(ctx, fd, level, option)
	}
	return value, errno
}

func (f *fallbackSystem) SockSetOpt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value SocketOptionValue) (errno Errno) {
	errno = f.primary.SockSetOpt(ctx, fd, level, option, value)
	if errno == ENOSYS {
		return f.secondary.SockSetOpt(ctx, fd, level, option, value)
	}
	return errno
}

func (f *fallbackSystem) SockLocalAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	addr, errno = f.primary.SockLocalAddress(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.SockLocalAddress(ctx, fd)
	}
	return addr, errno
}

func (f *fallbackSystem) SockRemoteAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	addr, errno = f.primary.SockRemoteAddress(ctx, fd)
	if errno == ENOSYS {
		return f.secondary.SockRemoteAddress(ctx, fd)
	}
	return addr, errno
}

func (f *fallbackSystem) SockAddressInfo(ctx context.Context, name, service string, hints AddressInfo, results []AddressInfo) (n int, errno Errno) {
	n, errno = f.primary.SockAddressInfo(ctx, name, service, hints, results)
	if errno == ENOSYS {
		return f.secondary.SockAddressInfo(ctx, name, service, hints, results)
	}
	return n, errno
}

func (f *fallbackSystem) Close(ctx context.Context) error {
	err := f.primary.Close(ctx)
	err2 := f.secondary.Close(ctx)
	if err == nil {
		err = err2
	}
	return err
}
