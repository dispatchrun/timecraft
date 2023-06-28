package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
)

// NewObserver creates a wasi.System that notifies callbacks before
// and/or after a system call.
func NewObserver(system System, before, after func(context.Context, Syscall)) System {
	return &observerSystem{system, before, after}
}

type observerSystem struct {
	system System
	before func(context.Context, Syscall)
	after  func(context.Context, Syscall)
}

func (o *observerSystem) ArgsSizesGet(ctx context.Context) (argCount int, stringBytes int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &ArgsSizesGetSyscall{argCount, stringBytes, errno})
	}
	argCount, stringBytes, errno = o.system.ArgsSizesGet(ctx)
	if o.after != nil {
		o.after(ctx, &ArgsSizesGetSyscall{argCount, stringBytes, errno})
	}
	return
}

func (o *observerSystem) ArgsGet(ctx context.Context) (args []string, errno Errno) {
	if o.before != nil {
		o.before(ctx, &ArgsGetSyscall{args, errno})
	}
	args, errno = o.system.ArgsGet(ctx)
	if o.after != nil {
		o.after(ctx, &ArgsGetSyscall{args, errno})
	}
	return
}

func (o *observerSystem) EnvironSizesGet(ctx context.Context) (envCount int, stringBytes int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &EnvironSizesGetSyscall{envCount, stringBytes, errno})
	}
	envCount, stringBytes, errno = o.system.EnvironSizesGet(ctx)
	if o.after != nil {
		o.after(ctx, &EnvironSizesGetSyscall{envCount, stringBytes, errno})
	}
	return
}

func (o *observerSystem) EnvironGet(ctx context.Context) (env []string, errno Errno) {
	if o.before != nil {
		o.before(ctx, &EnvironGetSyscall{env, errno})
	}
	env, errno = o.system.EnvironGet(ctx)
	if o.after != nil {
		o.after(ctx, &EnvironGetSyscall{env, errno})
	}
	return
}

func (o *observerSystem) ClockResGet(ctx context.Context, clockID ClockID) (timestamp Timestamp, errno Errno) {
	if o.before != nil {
		o.before(ctx, &ClockResGetSyscall{clockID, timestamp, errno})
	}
	timestamp, errno = o.system.ClockResGet(ctx, clockID)
	if o.after != nil {
		o.after(ctx, &ClockResGetSyscall{clockID, timestamp, errno})
	}
	return
}

func (o *observerSystem) ClockTimeGet(ctx context.Context, clockID ClockID, precision Timestamp) (timestamp Timestamp, errno Errno) {
	if o.before != nil {
		o.before(ctx, &ClockTimeGetSyscall{clockID, precision, timestamp, errno})
	}
	timestamp, errno = o.system.ClockTimeGet(ctx, clockID, precision)
	if o.after != nil {
		o.after(ctx, &ClockTimeGetSyscall{clockID, precision, timestamp, errno})
	}
	return
}

func (o *observerSystem) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDAdviseSyscall{fd, offset, length, advice, errno})
	}
	errno = o.system.FDAdvise(ctx, fd, offset, length, advice)
	if o.after != nil {
		o.after(ctx, &FDAdviseSyscall{fd, offset, length, advice, errno})
	}
	return
}

func (o *observerSystem) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDAllocateSyscall{fd, offset, length, errno})
	}
	errno = o.system.FDAllocate(ctx, fd, offset, length)
	if o.after != nil {
		o.after(ctx, &FDAllocateSyscall{fd, offset, length, errno})
	}
	return
}

func (o *observerSystem) FDClose(ctx context.Context, fd FD) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDCloseSyscall{fd, errno})
	}
	errno = o.system.FDClose(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDCloseSyscall{fd, errno})
	}
	return
}

func (o *observerSystem) FDDataSync(ctx context.Context, fd FD) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDDataSyncSyscall{fd, errno})
	}
	errno = o.system.FDDataSync(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDDataSyncSyscall{fd, errno})
	}
	return
}

func (o *observerSystem) FDStatGet(ctx context.Context, fd FD) (stat FDStat, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDStatGetSyscall{fd, stat, errno})
	}
	stat, errno = o.system.FDStatGet(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDStatGetSyscall{fd, stat, errno})
	}
	return
}

func (o *observerSystem) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDStatSetFlagsSyscall{fd, flags, errno})
	}
	errno = o.system.FDStatSetFlags(ctx, fd, flags)
	if o.after != nil {
		o.after(ctx, &FDStatSetFlagsSyscall{fd, flags, errno})
	}
	return
}

func (o *observerSystem) FDStatSetRights(ctx context.Context, fd FD, rightsBase Rights, rightsInheriting Rights) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDStatSetRightsSyscall{fd, rightsBase, rightsInheriting, errno})
	}
	errno = o.system.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	if o.after != nil {
		o.after(ctx, &FDStatSetRightsSyscall{fd, rightsBase, rightsInheriting, errno})
	}
	return
}

func (o *observerSystem) FDFileStatGet(ctx context.Context, fd FD) (stat FileStat, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDFileStatGetSyscall{fd, stat, errno})
	}
	stat, errno = o.system.FDFileStatGet(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDFileStatGetSyscall{fd, stat, errno})
	}
	return
}

func (o *observerSystem) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDFileStatSetSizeSyscall{fd, size, errno})
	}
	errno = o.system.FDFileStatSetSize(ctx, fd, size)
	if o.after != nil {
		o.after(ctx, &FDFileStatSetSizeSyscall{fd, size, errno})
	}
	return
}

func (o *observerSystem) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime Timestamp, modifyTime Timestamp, flags FSTFlags) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDFileStatSetTimesSyscall{fd, accessTime, modifyTime, flags, errno})
	}
	errno = o.system.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	if o.after != nil {
		o.after(ctx, &FDFileStatSetTimesSyscall{fd, accessTime, modifyTime, flags, errno})
	}
	return
}

func (o *observerSystem) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDPreadSyscall{fd, iovecs, offset, size, errno})
	}
	size, errno = o.system.FDPread(ctx, fd, iovecs, offset)
	if o.after != nil {
		o.after(ctx, &FDPreadSyscall{fd, iovecs, offset, size, errno})
	}
	return
}

func (o *observerSystem) FDPreStatGet(ctx context.Context, fd FD) (stat PreStat, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDPreStatGetSyscall{fd, stat, errno})
	}
	stat, errno = o.system.FDPreStatGet(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDPreStatGetSyscall{fd, stat, errno})
	}
	return
}

func (o *observerSystem) FDPreStatDirName(ctx context.Context, fd FD) (name string, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDPreStatDirNameSyscall{fd, name, errno})
	}
	name, errno = o.system.FDPreStatDirName(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDPreStatDirNameSyscall{fd, name, errno})
	}
	return
}

func (o *observerSystem) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDPwriteSyscall{fd, iovecs, offset, size, errno})
	}
	size, errno = o.system.FDPwrite(ctx, fd, iovecs, offset)
	if o.after != nil {
		o.after(ctx, &FDPwriteSyscall{fd, iovecs, offset, size, errno})
	}
	return
}

func (o *observerSystem) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDReadSyscall{fd, iovecs, size, errno})
	}
	size, errno = o.system.FDRead(ctx, fd, iovecs)
	if o.after != nil {
		o.after(ctx, &FDReadSyscall{fd, iovecs, size, errno})
	}
	return
}

func (o *observerSystem) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (n int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDReadDirSyscall{fd, entries, cookie, bufferSizeBytes, errno})
	}
	n, errno = o.system.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	if n >= 0 && n <= len(entries) {
		entries = entries[:n]
	} else {
		entries = entries[:0]
	}
	if o.after != nil {
		o.after(ctx, &FDReadDirSyscall{fd, entries, cookie, bufferSizeBytes, errno})
	}
	return
}

func (o *observerSystem) FDRenumber(ctx context.Context, from FD, to FD) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDRenumberSyscall{from, to, errno})
	}
	errno = o.system.FDRenumber(ctx, from, to)
	if o.after != nil {
		o.after(ctx, &FDRenumberSyscall{from, to, errno})
	}
	return
}

func (o *observerSystem) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (size FileSize, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDSeekSyscall{fd, offset, whence, size, errno})
	}
	size, errno = o.system.FDSeek(ctx, fd, offset, whence)
	if o.after != nil {
		o.after(ctx, &FDSeekSyscall{fd, offset, whence, size, errno})
	}
	return
}

func (o *observerSystem) FDSync(ctx context.Context, fd FD) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDSyncSyscall{fd, errno})
	}
	errno = o.system.FDSync(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDSyncSyscall{fd, errno})
	}
	return
}

func (o *observerSystem) FDTell(ctx context.Context, fd FD) (size FileSize, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDTellSyscall{fd, size, errno})
	}
	size, errno = o.system.FDTell(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDTellSyscall{fd, size, errno})
	}
	return
}

func (o *observerSystem) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDWriteSyscall{fd, iovecs, size, errno})
	}
	size, errno = o.system.FDWrite(ctx, fd, iovecs)
	if o.after != nil {
		o.after(ctx, &FDWriteSyscall{fd, iovecs, size, errno})
	}
	return
}

func (o *observerSystem) PathCreateDirectory(ctx context.Context, fd FD, path string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathCreateDirectorySyscall{fd, path, errno})
	}
	errno = o.system.PathCreateDirectory(ctx, fd, path)
	if o.after != nil {
		o.after(ctx, &PathCreateDirectorySyscall{fd, path, errno})
	}
	return
}

func (o *observerSystem) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (stat FileStat, errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathFileStatGetSyscall{fd, lookupFlags, path, stat, errno})
	}
	stat, errno = o.system.PathFileStatGet(ctx, fd, lookupFlags, path)
	if o.after != nil {
		o.after(ctx, &PathFileStatGetSyscall{fd, lookupFlags, path, stat, errno})
	}
	return
}

func (o *observerSystem) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime Timestamp, modifyTime Timestamp, flags FSTFlags) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathFileStatSetTimesSyscall{fd, lookupFlags, path, accessTime, modifyTime, flags, errno})
	}
	errno = o.system.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	if o.after != nil {
		o.after(ctx, &PathFileStatSetTimesSyscall{fd, lookupFlags, path, accessTime, modifyTime, flags, errno})
	}
	return
}

func (o *observerSystem) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathLinkSyscall{oldFD, oldFlags, oldPath, newFD, newPath, errno})
	}
	errno = o.system.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	if o.after != nil {
		o.after(ctx, &PathLinkSyscall{oldFD, oldFlags, oldPath, newFD, newPath, errno})
	}
	return
}

func (o *observerSystem) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase Rights, rightsInheriting Rights, fdFlags FDFlags) (newfd FD, errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathOpenSyscall{fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno})
	}
	newfd, errno = o.system.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if o.after != nil {
		o.after(ctx, &PathOpenSyscall{fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno})
	}
	return
}

func (o *observerSystem) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) (n int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathReadLinkSyscall{fd, path, buffer, errno})
	}
	n, errno = o.system.PathReadLink(ctx, fd, path, buffer)
	if n >= 0 && n <= len(buffer) {
		buffer = buffer[:n]
	} else {
		buffer = buffer[:0]
	}
	if o.after != nil {
		o.after(ctx, &PathReadLinkSyscall{fd, path, buffer, errno})
	}
	return
}

func (o *observerSystem) PathRemoveDirectory(ctx context.Context, fd FD, path string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathRemoveDirectorySyscall{fd, path, errno})
	}
	errno = o.system.PathRemoveDirectory(ctx, fd, path)
	if o.after != nil {
		o.after(ctx, &PathRemoveDirectorySyscall{fd, path, errno})
	}
	return
}

func (o *observerSystem) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathRenameSyscall{fd, oldPath, newFD, newPath, errno})
	}
	errno = o.system.PathRename(ctx, fd, oldPath, newFD, newPath)
	if o.after != nil {
		o.after(ctx, &PathRenameSyscall{fd, oldPath, newFD, newPath, errno})
	}
	return
}

func (o *observerSystem) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathSymlinkSyscall{oldPath, fd, newPath, errno})
	}
	errno = o.system.PathSymlink(ctx, oldPath, fd, newPath)
	if o.after != nil {
		o.after(ctx, &PathSymlinkSyscall{oldPath, fd, newPath, errno})
	}
	return
}

func (o *observerSystem) PathUnlinkFile(ctx context.Context, fd FD, path string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathUnlinkFileSyscall{fd, path, errno})
	}
	errno = o.system.PathUnlinkFile(ctx, fd, path)
	if o.after != nil {
		o.after(ctx, &PathUnlinkFileSyscall{fd, path, errno})
	}
	return
}

func (o *observerSystem) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (count int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &PollOneOffSyscall{subscriptions, events, errno})
	}
	count, errno = o.system.PollOneOff(ctx, subscriptions, events)
	if count >= 0 && count <= len(events) {
		events = events[:count]
	} else {
		events = events[:0]
	}
	if o.after != nil {
		o.after(ctx, &PollOneOffSyscall{subscriptions, events, errno})
	}
	return
}

func (o *observerSystem) ProcExit(ctx context.Context, exitCode ExitCode) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &ProcExitSyscall{exitCode, errno})
	}
	errno = o.system.ProcExit(ctx, exitCode)
	if o.after != nil {
		o.after(ctx, &ProcExitSyscall{exitCode, errno})
	}
	return
}

func (o *observerSystem) ProcRaise(ctx context.Context, signal Signal) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &ProcRaiseSyscall{signal, errno})
	}
	errno = o.system.ProcRaise(ctx, signal)
	if o.after != nil {
		o.after(ctx, &ProcRaiseSyscall{signal, errno})
	}
	return
}

func (o *observerSystem) SchedYield(ctx context.Context) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &SchedYieldSyscall{errno})
	}
	errno = o.system.SchedYield(ctx)
	if o.after != nil {
		o.after(ctx, &SchedYieldSyscall{errno})
	}
	return
}

func (o *observerSystem) RandomGet(ctx context.Context, b []byte) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &RandomGetSyscall{b, errno})
	}
	errno = o.system.RandomGet(ctx, b)
	if o.after != nil {
		o.after(ctx, &RandomGetSyscall{b, errno})
	}
	return
}

func (o *observerSystem) SockAccept(ctx context.Context, fd FD, flags FDFlags) (newfd FD, peer, addr SocketAddress, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockAcceptSyscall{fd, flags, newfd, peer, addr, errno})
	}
	newfd, peer, addr, errno = o.system.SockAccept(ctx, fd, flags)
	if o.after != nil {
		o.after(ctx, &SockAcceptSyscall{fd, flags, newfd, peer, addr, errno})
	}
	return
}

func (o *observerSystem) SockShutdown(ctx context.Context, fd FD, flags SDFlags) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockShutdownSyscall{fd, flags, errno})
	}
	errno = o.system.SockShutdown(ctx, fd, flags)
	if o.after != nil {
		o.after(ctx, &SockShutdownSyscall{fd, flags, errno})
	}
	return
}

func (o *observerSystem) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (size Size, oflags ROFlags, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockRecvSyscall{fd, iovecs, iflags, size, oflags, errno})
	}
	size, oflags, errno = o.system.SockRecv(ctx, fd, iovecs, iflags)
	if o.after != nil {
		o.after(ctx, &SockRecvSyscall{fd, iovecs, iflags, size, oflags, errno})
	}
	return
}

func (o *observerSystem) SockSend(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockSendSyscall{fd, iovecs, iflags, size, errno})
	}
	size, errno = o.system.SockSend(ctx, fd, iovecs, iflags)
	if o.after != nil {
		o.after(ctx, &SockSendSyscall{fd, iovecs, iflags, size, errno})
	}
	return
}

func (o *observerSystem) SockOpen(ctx context.Context, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase Rights, rightsInheriting Rights) (fd FD, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockOpenSyscall{family, socketType, protocol, rightsBase, rightsInheriting, fd, errno})
	}
	fd, errno = o.system.SockOpen(ctx, family, socketType, protocol, rightsBase, rightsInheriting)
	if o.after != nil {
		o.after(ctx, &SockOpenSyscall{family, socketType, protocol, rightsBase, rightsInheriting, fd, errno})
	}
	return
}

func (o *observerSystem) SockBind(ctx context.Context, fd FD, bind SocketAddress) (addr SocketAddress, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockBindSyscall{fd, bind, addr, errno})
	}
	addr, errno = o.system.SockBind(ctx, fd, bind)
	if o.after != nil {
		o.after(ctx, &SockBindSyscall{fd, bind, addr, errno})
	}
	return
}

func (o *observerSystem) SockConnect(ctx context.Context, fd FD, peer SocketAddress) (addr SocketAddress, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockConnectSyscall{fd, peer, addr, errno})
	}
	addr, errno = o.system.SockConnect(ctx, fd, peer)
	if o.after != nil {
		o.after(ctx, &SockConnectSyscall{fd, peer, addr, errno})
	}
	return
}

func (o *observerSystem) SockListen(ctx context.Context, fd FD, backlog int) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockListenSyscall{fd, backlog, errno})
	}
	errno = o.system.SockListen(ctx, fd, backlog)
	if o.after != nil {
		o.after(ctx, &SockListenSyscall{fd, backlog, errno})
	}
	return
}

func (o *observerSystem) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockSendToSyscall{fd, iovecs, iflags, addr, size, errno})
	}
	size, errno = o.system.SockSendTo(ctx, fd, iovecs, iflags, addr)
	if o.after != nil {
		o.after(ctx, &SockSendToSyscall{fd, iovecs, iflags, addr, size, errno})
	}
	return
}

func (o *observerSystem) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (size Size, oflags ROFlags, addr SocketAddress, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockRecvFromSyscall{fd, iovecs, iflags, size, oflags, addr, errno})
	}
	size, oflags, addr, errno = o.system.SockRecvFrom(ctx, fd, iovecs, iflags)
	if o.after != nil {
		o.after(ctx, &SockRecvFromSyscall{fd, iovecs, iflags, size, oflags, addr, errno})
	}
	return
}

func (o *observerSystem) SockGetOpt(ctx context.Context, fd FD, option SocketOption) (value SocketOptionValue, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockGetOptSyscall{fd, option, value, errno})
	}
	value, errno = o.system.SockGetOpt(ctx, fd, option)
	if o.after != nil {
		o.after(ctx, &SockGetOptSyscall{fd, option, value, errno})
	}
	return
}

func (o *observerSystem) SockSetOpt(ctx context.Context, fd FD, option SocketOption, value SocketOptionValue) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockSetOptSyscall{fd, option, value, errno})
	}
	errno = o.system.SockSetOpt(ctx, fd, option, value)
	if o.after != nil {
		o.after(ctx, &SockSetOptSyscall{fd, option, value, errno})
	}
	return
}

func (o *observerSystem) SockLocalAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockLocalAddressSyscall{fd, addr, errno})
	}
	addr, errno = o.system.SockLocalAddress(ctx, fd)
	if o.after != nil {
		o.after(ctx, &SockLocalAddressSyscall{fd, addr, errno})
	}
	return
}

func (o *observerSystem) SockRemoteAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockRemoteAddressSyscall{fd, addr, errno})
	}
	addr, errno = o.system.SockRemoteAddress(ctx, fd)
	if o.after != nil {
		o.after(ctx, &SockRemoteAddressSyscall{fd, addr, errno})
	}
	return
}

func (o *observerSystem) SockAddressInfo(ctx context.Context, name, service string, hints AddressInfo, results []AddressInfo) (n int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockAddressInfoSyscall{name, service, hints, results, errno})
	}
	n, errno = o.system.SockAddressInfo(ctx, name, service, hints, results)
	if n >= 0 && n <= len(results) {
		results = results[:n]
	} else {
		results = results[:0]
	}
	if o.after != nil {
		o.before(ctx, &SockAddressInfoSyscall{name, service, hints, results, errno})
	}
	return
}

func (o *observerSystem) Close(ctx context.Context) error {
	return o.system.Close(ctx)
}
