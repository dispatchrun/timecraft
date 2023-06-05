package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
)

// Observer wraps a wasi.System for observation.
type Observer struct {
	System

	before ObserverCallback
	after  ObserverCallback
}

type ObserverCallback func(context.Context, Syscall)

// NewObserver creates an Observer.
func NewObserver(system System, before, after ObserverCallback) *Observer {
	return &Observer{system, before, after}
}

var _ System = (*Observer)(nil)
var _ SocketsExtension = (*Observer)(nil)

func (o *Observer) ArgsSizesGet(ctx context.Context) (argCount int, stringBytes int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &ArgsSizesGetSyscall{argCount, stringBytes, errno})
	}
	argCount, stringBytes, errno = o.System.ArgsSizesGet(ctx)
	if o.after != nil {
		o.after(ctx, &ArgsSizesGetSyscall{argCount, stringBytes, errno})
	}
	return
}

func (o *Observer) ArgsGet(ctx context.Context) (args []string, errno Errno) {
	if o.before != nil {
		o.before(ctx, &ArgsGetSyscall{args, errno})
	}
	args, errno = o.System.ArgsGet(ctx)
	if o.after != nil {
		o.after(ctx, &ArgsGetSyscall{args, errno})
	}
	return
}

func (o *Observer) EnvironSizesGet(ctx context.Context) (envCount int, stringBytes int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &EnvironSizesGetSyscall{envCount, stringBytes, errno})
	}
	envCount, stringBytes, errno = o.System.EnvironSizesGet(ctx)
	if o.after != nil {
		o.after(ctx, &EnvironSizesGetSyscall{envCount, stringBytes, errno})
	}
	return
}

func (o *Observer) EnvironGet(ctx context.Context) (env []string, errno Errno) {
	if o.before != nil {
		o.before(ctx, &EnvironGetSyscall{env, errno})
	}
	env, errno = o.System.EnvironGet(ctx)
	if o.after != nil {
		o.after(ctx, &EnvironGetSyscall{env, errno})
	}
	return
}

func (o *Observer) ClockResGet(ctx context.Context, clockID ClockID) (timestamp Timestamp, errno Errno) {
	if o.before != nil {
		o.before(ctx, &ClockResGetSyscall{clockID, timestamp, errno})
	}
	timestamp, errno = o.System.ClockResGet(ctx, clockID)
	if o.after != nil {
		o.after(ctx, &ClockResGetSyscall{clockID, timestamp, errno})
	}
	return
}

func (o *Observer) ClockTimeGet(ctx context.Context, clockID ClockID, precision Timestamp) (timestamp Timestamp, errno Errno) {
	if o.before != nil {
		o.before(ctx, &ClockTimeGetSyscall{clockID, precision, timestamp, errno})
	}
	timestamp, errno = o.System.ClockTimeGet(ctx, clockID, precision)
	if o.after != nil {
		o.after(ctx, &ClockTimeGetSyscall{clockID, precision, timestamp, errno})
	}
	return
}

func (o *Observer) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDAdviseSyscall{fd, offset, length, advice, errno})
	}
	errno = o.System.FDAdvise(ctx, fd, offset, length, advice)
	if o.after != nil {
		o.after(ctx, &FDAdviseSyscall{fd, offset, length, advice, errno})
	}
	return
}

func (o *Observer) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDAllocateSyscall{fd, offset, length, errno})
	}
	errno = o.System.FDAllocate(ctx, fd, offset, length)
	if o.after != nil {
		o.after(ctx, &FDAllocateSyscall{fd, offset, length, errno})
	}
	return
}

func (o *Observer) FDClose(ctx context.Context, fd FD) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDCloseSyscall{fd, errno})
	}
	errno = o.System.FDClose(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDCloseSyscall{fd, errno})
	}
	return
}

func (o *Observer) FDDataSync(ctx context.Context, fd FD) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDDataSyncSyscall{fd, errno})
	}
	errno = o.System.FDDataSync(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDDataSyncSyscall{fd, errno})
	}
	return
}

func (o *Observer) FDStatGet(ctx context.Context, fd FD) (stat FDStat, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDStatGetSyscall{fd, stat, errno})
	}
	stat, errno = o.System.FDStatGet(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDStatGetSyscall{fd, stat, errno})
	}
	return
}

func (o *Observer) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDStatSetFlagsSyscall{fd, flags, errno})
	}
	errno = o.System.FDStatSetFlags(ctx, fd, flags)
	if o.after != nil {
		o.after(ctx, &FDStatSetFlagsSyscall{fd, flags, errno})
	}
	return
}

func (o *Observer) FDStatSetRights(ctx context.Context, fd FD, rightsBase Rights, rightsInheriting Rights) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDStatSetRightsSyscall{fd, rightsBase, rightsInheriting, errno})
	}
	errno = o.System.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	if o.after != nil {
		o.after(ctx, &FDStatSetRightsSyscall{fd, rightsBase, rightsInheriting, errno})
	}
	return
}

func (o *Observer) FDFileStatGet(ctx context.Context, fd FD) (stat FileStat, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDFileStatGetSyscall{fd, stat, errno})
	}
	stat, errno = o.System.FDFileStatGet(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDFileStatGetSyscall{fd, stat, errno})
	}
	return
}

func (o *Observer) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDFileStatSetSizeSyscall{fd, size, errno})
	}
	errno = o.System.FDFileStatSetSize(ctx, fd, size)
	if o.after != nil {
		o.after(ctx, &FDFileStatSetSizeSyscall{fd, size, errno})
	}
	return
}

func (o *Observer) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime Timestamp, modifyTime Timestamp, flags FSTFlags) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDFileStatSetTimesSyscall{fd, accessTime, modifyTime, flags, errno})
	}
	errno = o.System.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	if o.after != nil {
		o.after(ctx, &FDFileStatSetTimesSyscall{fd, accessTime, modifyTime, flags, errno})
	}
	return
}

func (o *Observer) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDPreadSyscall{fd, iovecs, offset, size, errno})
	}
	size, errno = o.System.FDPread(ctx, fd, iovecs, offset)
	if o.after != nil {
		o.after(ctx, &FDPreadSyscall{fd, iovecs, offset, size, errno})
	}
	return
}

func (o *Observer) FDPreStatGet(ctx context.Context, fd FD) (stat PreStat, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDPreStatGetSyscall{fd, stat, errno})
	}
	stat, errno = o.System.FDPreStatGet(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDPreStatGetSyscall{fd, stat, errno})
	}
	return
}

func (o *Observer) FDPreStatDirName(ctx context.Context, fd FD) (name string, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDPreStatDirNameSyscall{fd, name, errno})
	}
	name, errno = o.System.FDPreStatDirName(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDPreStatDirNameSyscall{fd, name, errno})
	}
	return
}

func (o *Observer) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDPwriteSyscall{fd, iovecs, offset, size, errno})
	}
	size, errno = o.System.FDPwrite(ctx, fd, iovecs, offset)
	if o.after != nil {
		o.after(ctx, &FDPwriteSyscall{fd, iovecs, offset, size, errno})
	}
	return
}

func (o *Observer) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDReadSyscall{fd, iovecs, size, errno})
	}
	size, errno = o.System.FDRead(ctx, fd, iovecs)
	if o.after != nil {
		o.after(ctx, &FDReadSyscall{fd, iovecs, size, errno})
	}
	return
}

func (o *Observer) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (count int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDReadDirSyscall{fd, entries, cookie, bufferSizeBytes, count, errno})
	}
	count, errno = o.System.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	if o.after != nil {
		o.after(ctx, &FDReadDirSyscall{fd, entries, cookie, bufferSizeBytes, count, errno})
	}
	return
}

func (o *Observer) FDRenumber(ctx context.Context, from FD, to FD) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDRenumberSyscall{from, to, errno})
	}
	errno = o.System.FDRenumber(ctx, from, to)
	if o.after != nil {
		o.after(ctx, &FDRenumberSyscall{from, to, errno})
	}
	return
}

func (o *Observer) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (size FileSize, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDSeekSyscall{fd, offset, whence, size, errno})
	}
	size, errno = o.System.FDSeek(ctx, fd, offset, whence)
	if o.after != nil {
		o.after(ctx, &FDSeekSyscall{fd, offset, whence, size, errno})
	}
	return
}

func (o *Observer) FDSync(ctx context.Context, fd FD) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDSyncSyscall{fd, errno})
	}
	errno = o.System.FDSync(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDSyncSyscall{fd, errno})
	}
	return
}

func (o *Observer) FDTell(ctx context.Context, fd FD) (size FileSize, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDTellSyscall{fd, size, errno})
	}
	size, errno = o.System.FDTell(ctx, fd)
	if o.after != nil {
		o.after(ctx, &FDTellSyscall{fd, size, errno})
	}
	return
}

func (o *Observer) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &FDWriteSyscall{fd, iovecs, size, errno})
	}
	size, errno = o.System.FDWrite(ctx, fd, iovecs)
	if o.after != nil {
		o.after(ctx, &FDWriteSyscall{fd, iovecs, size, errno})
	}
	return
}

func (o *Observer) PathCreateDirectory(ctx context.Context, fd FD, path string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathCreateDirectorySyscall{fd, path, errno})
	}
	errno = o.System.PathCreateDirectory(ctx, fd, path)
	if o.after != nil {
		o.after(ctx, &PathCreateDirectorySyscall{fd, path, errno})
	}
	return
}

func (o *Observer) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (stat FileStat, errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathFileStatGetSyscall{fd, lookupFlags, path, stat, errno})
	}
	stat, errno = o.System.PathFileStatGet(ctx, fd, lookupFlags, path)
	if o.after != nil {
		o.after(ctx, &PathFileStatGetSyscall{fd, lookupFlags, path, stat, errno})
	}
	return
}

func (o *Observer) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime Timestamp, modifyTime Timestamp, flags FSTFlags) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathFileStatSetTimesSyscall{fd, lookupFlags, path, accessTime, modifyTime, flags, errno})
	}
	errno = o.System.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	if o.after != nil {
		o.after(ctx, &PathFileStatSetTimesSyscall{fd, lookupFlags, path, accessTime, modifyTime, flags, errno})
	}
	return
}

func (o *Observer) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathLinkSyscall{oldFD, oldFlags, oldPath, newFD, newPath, errno})
	}
	errno = o.System.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	if o.after != nil {
		o.after(ctx, &PathLinkSyscall{oldFD, oldFlags, oldPath, newFD, newPath, errno})
	}
	return
}

func (o *Observer) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase Rights, rightsInheriting Rights, fdFlags FDFlags) (newfd FD, errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathOpenSyscall{fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno})
	}
	newfd, errno = o.System.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if o.after != nil {
		o.after(ctx, &PathOpenSyscall{fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno})
	}
	return
}

func (o *Observer) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) (output []byte, errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathReadLinkSyscall{fd, path, buffer, errno})
	}
	output, errno = o.System.PathReadLink(ctx, fd, path, buffer)
	if o.after != nil {
		o.after(ctx, &PathReadLinkSyscall{fd, path, output, errno})
	}
	return
}

func (o *Observer) PathRemoveDirectory(ctx context.Context, fd FD, path string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathRemoveDirectorySyscall{fd, path, errno})
	}
	errno = o.System.PathRemoveDirectory(ctx, fd, path)
	if o.after != nil {
		o.after(ctx, &PathRemoveDirectorySyscall{fd, path, errno})
	}
	return
}

func (o *Observer) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathRenameSyscall{fd, oldPath, newFD, newPath, errno})
	}
	errno = o.System.PathRename(ctx, fd, oldPath, newFD, newPath)
	if o.after != nil {
		o.after(ctx, &PathRenameSyscall{fd, oldPath, newFD, newPath, errno})
	}
	return
}

func (o *Observer) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathSymlinkSyscall{oldPath, fd, newPath, errno})
	}
	errno = o.System.PathSymlink(ctx, oldPath, fd, newPath)
	if o.after != nil {
		o.after(ctx, &PathSymlinkSyscall{oldPath, fd, newPath, errno})
	}
	return
}

func (o *Observer) PathUnlinkFile(ctx context.Context, fd FD, path string) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &PathUnlinkFileSyscall{fd, path, errno})
	}
	errno = o.System.PathUnlinkFile(ctx, fd, path)
	if o.after != nil {
		o.after(ctx, &PathUnlinkFileSyscall{fd, path, errno})
	}
	return
}

func (o *Observer) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (count int, errno Errno) {
	if o.before != nil {
		o.before(ctx, &PollOneOffSyscall{subscriptions, events, errno})
	}
	count, errno = o.System.PollOneOff(ctx, subscriptions, events)
	if o.after != nil {
		o.after(ctx, &PollOneOffSyscall{subscriptions, events[:count], errno})
	}
	return
}

func (o *Observer) ProcExit(ctx context.Context, exitCode ExitCode) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &ProcExitSyscall{exitCode, errno})
	}
	errno = o.System.ProcExit(ctx, exitCode)
	if o.after != nil {
		o.after(ctx, &ProcExitSyscall{exitCode, errno})
	}
	return
}

func (o *Observer) ProcRaise(ctx context.Context, signal Signal) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &ProcRaiseSyscall{signal, errno})
	}
	errno = o.System.ProcRaise(ctx, signal)
	if o.after != nil {
		o.after(ctx, &ProcRaiseSyscall{signal, errno})
	}
	return
}

func (o *Observer) SchedYield(ctx context.Context) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &SchedYieldSyscall{errno})
	}
	errno = o.System.SchedYield(ctx)
	if o.after != nil {
		o.after(ctx, &SchedYieldSyscall{errno})
	}
	return
}

func (o *Observer) RandomGet(ctx context.Context, b []byte) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &RandomGetSyscall{b, errno})
	}
	errno = o.System.RandomGet(ctx, b)
	if o.after != nil {
		o.after(ctx, &RandomGetSyscall{b, errno})
	}
	return
}

func (o *Observer) SockAccept(ctx context.Context, fd FD, flags FDFlags) (newfd FD, peer, addr SocketAddress, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockAcceptSyscall{fd, flags, newfd, peer, addr, errno})
	}
	newfd, peer, addr, errno = o.System.SockAccept(ctx, fd, flags)
	if o.after != nil {
		o.after(ctx, &SockAcceptSyscall{fd, flags, newfd, peer, addr, errno})
	}
	return
}

func (o *Observer) SockShutdown(ctx context.Context, fd FD, flags SDFlags) (errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockShutdownSyscall{fd, flags, errno})
	}
	errno = o.System.SockShutdown(ctx, fd, flags)
	if o.after != nil {
		o.after(ctx, &SockShutdownSyscall{fd, flags, errno})
	}
	return
}

func (o *Observer) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (size Size, oflags ROFlags, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockRecvSyscall{fd, iovecs, iflags, size, oflags, errno})
	}
	size, oflags, errno = o.System.SockRecv(ctx, fd, iovecs, iflags)
	if o.after != nil {
		o.after(ctx, &SockRecvSyscall{fd, iovecs, iflags, size, oflags, errno})
	}
	return
}

func (o *Observer) SockSend(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags) (size Size, errno Errno) {
	if o.before != nil {
		o.before(ctx, &SockSendSyscall{fd, iovecs, iflags, size, errno})
	}
	size, errno = o.System.SockSend(ctx, fd, iovecs, iflags)
	if o.after != nil {
		o.after(ctx, &SockSendSyscall{fd, iovecs, iflags, size, errno})
	}
	return
}

func (o *Observer) SockOpen(ctx context.Context, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase Rights, rightsInheriting Rights) (fd FD, errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return -1, ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockOpenSyscall{family, socketType, protocol, rightsBase, rightsInheriting, fd, errno})
	}
	fd, errno = se.SockOpen(ctx, family, socketType, protocol, rightsBase, rightsInheriting)
	if o.after != nil {
		o.after(ctx, &SockOpenSyscall{family, socketType, protocol, rightsBase, rightsInheriting, fd, errno})
	}
	return
}

func (o *Observer) SockBind(ctx context.Context, fd FD, bind SocketAddress) (addr SocketAddress, errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockBindSyscall{fd, bind, addr, errno})
	}
	addr, errno = se.SockBind(ctx, fd, bind)
	if o.after != nil {
		o.after(ctx, &SockBindSyscall{fd, bind, addr, errno})
	}
	return
}

func (o *Observer) SockConnect(ctx context.Context, fd FD, peer SocketAddress) (addr SocketAddress, errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockConnectSyscall{fd, peer, addr, errno})
	}
	addr, errno = se.SockConnect(ctx, fd, peer)
	if o.after != nil {
		o.after(ctx, &SockConnectSyscall{fd, peer, addr, errno})
	}
	return
}

func (o *Observer) SockListen(ctx context.Context, fd FD, backlog int) (errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockListenSyscall{fd, backlog, errno})
	}
	errno = se.SockListen(ctx, fd, backlog)
	if o.after != nil {
		o.after(ctx, &SockListenSyscall{fd, backlog, errno})
	}
	return
}

func (o *Observer) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress) (size Size, errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return 0, ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockSendToSyscall{fd, iovecs, iflags, addr, size, errno})
	}
	size, errno = se.SockSendTo(ctx, fd, iovecs, iflags, addr)
	if o.after != nil {
		o.after(ctx, &SockSendToSyscall{fd, iovecs, iflags, addr, size, errno})
	}
	return
}

func (o *Observer) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (size Size, oflags ROFlags, addr SocketAddress, errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return 0, 0, nil, ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockRecvFromSyscall{fd, iovecs, iflags, size, oflags, addr, errno})
	}
	size, oflags, addr, errno = se.SockRecvFrom(ctx, fd, iovecs, iflags)
	if o.after != nil {
		o.after(ctx, &SockRecvFromSyscall{fd, iovecs, iflags, size, oflags, addr, errno})
	}
	return
}

func (o *Observer) SockGetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (value int, errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return -1, ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockGetOptIntSyscall{fd, level, option, value, errno})
	}
	value, errno = se.SockGetOptInt(ctx, fd, level, option)
	if o.after != nil {
		o.after(ctx, &SockGetOptIntSyscall{fd, level, option, value, errno})
	}
	return
}

func (o *Observer) SockSetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value int) (errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockSetOptIntSyscall{fd, level, option, value, errno})
	}
	errno = se.SockSetOptInt(ctx, fd, level, option, value)
	if o.after != nil {
		o.after(ctx, &SockSetOptIntSyscall{fd, level, option, value, errno})
	}
	return
}

func (o *Observer) SockLocalAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockLocalAddressSyscall{fd, addr, errno})
	}
	addr, errno = se.SockLocalAddress(ctx, fd)
	if o.after != nil {
		o.after(ctx, &SockLocalAddressSyscall{fd, addr, errno})
	}
	return
}

func (o *Observer) SockRemoteAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	if o.before != nil {
		o.before(ctx, &SockRemoteAddressSyscall{fd, addr, errno})
	}
	addr, errno = se.SockRemoteAddress(ctx, fd)
	if o.after != nil {
		o.after(ctx, &SockRemoteAddressSyscall{fd, addr, errno})
	}
	return
}
