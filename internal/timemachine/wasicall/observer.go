package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
)

// Observer wraps a wasi.System for observation.
//
// The provided callback receives a Syscall instance with details
// about a system call each time one is made. The callback is called after
// the system call returns but before control is returned to the guest.
// This allows the observer to see both system call inputs and outputs.
// It also allows for other action to be taken prior to system calls
// returning, for example pausing when providing an interactive debugging
// experience.
//
// The callback must not retain the syscall or any of its fields.
type Observer struct {
	System

	observe func(context.Context, Syscall)
}

// NewObserver creates an Observer.
func NewObserver(system System, observe func(context.Context, Syscall)) *Observer {
	return &Observer{system, observe}
}

var _ System = (*Observer)(nil)
var _ SocketsExtension = (*Observer)(nil)

func (o *Observer) ArgsSizesGet(ctx context.Context) (int, int, Errno) {
	argCount, stringBytes, errno := o.System.ArgsSizesGet(ctx)
	o.observe(ctx, &ArgsSizesGetSyscall{argCount, stringBytes, errno})
	return argCount, stringBytes, errno
}

func (o *Observer) ArgsGet(ctx context.Context) ([]string, Errno) {
	args, errno := o.System.ArgsGet(ctx)
	o.observe(ctx, &ArgsGetSyscall{args, errno})
	return args, errno
}

func (o *Observer) EnvironSizesGet(ctx context.Context) (int, int, Errno) {
	envCount, stringBytes, errno := o.System.EnvironSizesGet(ctx)
	o.observe(ctx, &EnvironSizesGetSyscall{envCount, stringBytes, errno})
	return envCount, stringBytes, errno
}

func (o *Observer) EnvironGet(ctx context.Context) ([]string, Errno) {
	env, errno := o.System.EnvironGet(ctx)
	o.observe(ctx, &EnvironGetSyscall{env, errno})
	return env, errno
}

func (o *Observer) ClockResGet(ctx context.Context, clockID ClockID) (Timestamp, Errno) {
	timestamp, errno := o.System.ClockResGet(ctx, clockID)
	o.observe(ctx, &ClockResGetSyscall{clockID, timestamp, errno})
	return timestamp, errno
}

func (o *Observer) ClockTimeGet(ctx context.Context, clockID ClockID, precision Timestamp) (Timestamp, Errno) {
	timestamp, errno := o.System.ClockTimeGet(ctx, clockID, precision)
	o.observe(ctx, &ClockTimeGetSyscall{clockID, precision, timestamp, errno})
	return timestamp, errno
}

func (o *Observer) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	errno := o.System.FDAdvise(ctx, fd, offset, length, advice)
	o.observe(ctx, &FDAdviseSyscall{fd, offset, length, advice, errno})
	return errno
}

func (o *Observer) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	errno := o.System.FDAllocate(ctx, fd, offset, length)
	o.observe(ctx, &FDAllocateSyscall{fd, offset, length, errno})
	return errno
}

func (o *Observer) FDClose(ctx context.Context, fd FD) Errno {
	errno := o.System.FDClose(ctx, fd)
	o.observe(ctx, &FDCloseSyscall{fd, errno})
	return errno
}

func (o *Observer) FDDataSync(ctx context.Context, fd FD) Errno {
	errno := o.System.FDDataSync(ctx, fd)
	o.observe(ctx, &FDDataSyncSyscall{fd, errno})
	return errno
}

func (o *Observer) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	stat, errno := o.System.FDStatGet(ctx, fd)
	o.observe(ctx, &FDStatGetSyscall{fd, stat, errno})
	return stat, errno
}

func (o *Observer) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	errno := o.System.FDStatSetFlags(ctx, fd, flags)
	o.observe(ctx, &FDStatSetFlagsSyscall{fd, flags, errno})
	return errno
}

func (o *Observer) FDStatSetRights(ctx context.Context, fd FD, rightsBase Rights, rightsInheriting Rights) Errno {
	errno := o.System.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	o.observe(ctx, &FDStatSetRightsSyscall{fd, rightsBase, rightsInheriting, errno})
	return errno
}

func (o *Observer) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	stat, errno := o.System.FDFileStatGet(ctx, fd)
	o.observe(ctx, &FDFileStatGetSyscall{fd, stat, errno})
	return stat, errno
}

func (o *Observer) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	errno := o.System.FDFileStatSetSize(ctx, fd, size)
	o.observe(ctx, &FDFileStatSetSizeSyscall{fd, size, errno})
	return errno
}

func (o *Observer) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime Timestamp, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := o.System.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	o.observe(ctx, &FDFileStatSetTimesSyscall{fd, accessTime, modifyTime, flags, errno})
	return errno
}

func (o *Observer) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	size, errno := o.System.FDPread(ctx, fd, iovecs, offset)
	o.observe(ctx, &FDPreadSyscall{fd, iovecs, offset, size, errno})
	return size, errno
}

func (o *Observer) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	stat, errno := o.System.FDPreStatGet(ctx, fd)
	o.observe(ctx, &FDPreStatGetSyscall{fd, stat, errno})
	return stat, errno
}

func (o *Observer) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	name, errno := o.System.FDPreStatDirName(ctx, fd)
	o.observe(ctx, &FDPreStatDirNameSyscall{fd, name, errno})
	return name, errno
}

func (o *Observer) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	size, errno := o.System.FDPwrite(ctx, fd, iovecs, offset)
	o.observe(ctx, &FDPwriteSyscall{fd, iovecs, offset, size, errno})
	return size, errno
}

func (o *Observer) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	size, errno := o.System.FDRead(ctx, fd, iovecs)
	o.observe(ctx, &FDReadSyscall{fd, iovecs, size, errno})
	return size, errno
}

func (o *Observer) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	count, errno := o.System.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	o.observe(ctx, &FDReadDirSyscall{fd, entries, cookie, bufferSizeBytes, count, errno})
	return count, errno
}

func (o *Observer) FDRenumber(ctx context.Context, from FD, to FD) Errno {
	errno := o.System.FDRenumber(ctx, from, to)
	o.observe(ctx, &FDRenumberSyscall{from, to, errno})
	return errno
}

func (o *Observer) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	size, errno := o.System.FDSeek(ctx, fd, offset, whence)
	o.observe(ctx, &FDSeekSyscall{fd, offset, whence, size, errno})
	return size, errno
}

func (o *Observer) FDSync(ctx context.Context, fd FD) Errno {
	errno := o.System.FDSync(ctx, fd)
	o.observe(ctx, &FDSyncSyscall{fd, errno})
	return errno
}

func (o *Observer) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	size, errno := o.System.FDTell(ctx, fd)
	o.observe(ctx, &FDTellSyscall{fd, size, errno})
	return size, errno
}

func (o *Observer) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	size, errno := o.System.FDWrite(ctx, fd, iovecs)
	o.observe(ctx, &FDWriteSyscall{fd, iovecs, size, errno})
	return size, errno
}

func (o *Observer) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := o.System.PathCreateDirectory(ctx, fd, path)
	o.observe(ctx, &PathCreateDirectorySyscall{fd, path, errno})
	return errno
}

func (o *Observer) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	stat, errno := o.System.PathFileStatGet(ctx, fd, lookupFlags, path)
	o.observe(ctx, &PathFileStatGetSyscall{fd, lookupFlags, path, stat, errno})
	return stat, errno
}

func (o *Observer) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime Timestamp, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := o.System.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	o.observe(ctx, &PathFileStatSetTimesSyscall{fd, lookupFlags, path, accessTime, modifyTime, flags, errno})
	return errno
}

func (o *Observer) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	errno := o.System.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	o.observe(ctx, &PathLinkSyscall{oldFD, oldFlags, oldPath, newFD, newPath, errno})
	return errno
}

func (o *Observer) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase Rights, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	newfd, errno := o.System.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	o.observe(ctx, &PathOpenSyscall{fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno})
	return newfd, errno
}

func (o *Observer) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) ([]byte, Errno) {
	output, errno := o.System.PathReadLink(ctx, fd, path, buffer)
	o.observe(ctx, &PathReadLinkSyscall{fd, path, output, errno})
	return output, errno
}

func (o *Observer) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := o.System.PathRemoveDirectory(ctx, fd, path)
	o.observe(ctx, &PathRemoveDirectorySyscall{fd, path, errno})
	return errno
}

func (o *Observer) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	errno := o.System.PathRename(ctx, fd, oldPath, newFD, newPath)
	o.observe(ctx, &PathRenameSyscall{fd, oldPath, newFD, newPath, errno})
	return errno
}

func (o *Observer) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	errno := o.System.PathSymlink(ctx, oldPath, fd, newPath)
	o.observe(ctx, &PathSymlinkSyscall{oldPath, fd, newPath, errno})
	return errno
}

func (o *Observer) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	errno := o.System.PathUnlinkFile(ctx, fd, path)
	o.observe(ctx, &PathUnlinkFileSyscall{fd, path, errno})
	return errno
}

func (o *Observer) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	count, errno := o.System.PollOneOff(ctx, subscriptions, events)
	o.observe(ctx, &PollOneOffSyscall{subscriptions, events[:count], errno})
	return count, errno
}

func (o *Observer) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	errno := o.System.ProcExit(ctx, exitCode)
	o.observe(ctx, &ProcExitSyscall{exitCode, errno})
	return errno
}

func (o *Observer) ProcRaise(ctx context.Context, signal Signal) Errno {
	errno := o.System.ProcRaise(ctx, signal)
	o.observe(ctx, &ProcRaiseSyscall{signal, errno})
	return errno
}

func (o *Observer) SchedYield(ctx context.Context) Errno {
	errno := o.System.SchedYield(ctx)
	o.observe(ctx, &SchedYieldSyscall{errno})
	return errno
}

func (o *Observer) RandomGet(ctx context.Context, b []byte) Errno {
	errno := o.System.RandomGet(ctx, b)
	o.observe(ctx, &RandomGetSyscall{b, errno})
	return errno
}

func (o *Observer) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, Errno) {
	newfd, errno := o.System.SockAccept(ctx, fd, flags)
	o.observe(ctx, &SockAcceptSyscall{fd, flags, newfd, errno})
	return newfd, errno
}

func (o *Observer) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	errno := o.System.SockShutdown(ctx, fd, flags)
	o.observe(ctx, &SockShutdownSyscall{fd, flags, errno})
	return errno
}

func (o *Observer) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, Errno) {
	size, oflags, errno := o.System.SockRecv(ctx, fd, iovecs, iflags)
	o.observe(ctx, &SockRecvSyscall{fd, iovecs, iflags, size, oflags, errno})
	return size, oflags, errno
}

func (o *Observer) SockSend(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags) (Size, Errno) {
	size, errno := o.System.SockSend(ctx, fd, iovecs, iflags)
	o.observe(ctx, &SockSendSyscall{fd, iovecs, iflags, size, errno})
	return size, errno
}

func (o *Observer) SockOpen(ctx context.Context, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase Rights, rightsInheriting Rights) (FD, Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return -1, ENOSYS
	}
	fd, errno := se.SockOpen(ctx, family, socketType, protocol, rightsBase, rightsInheriting)
	o.observe(ctx, &SockOpenSyscall{family, socketType, protocol, rightsBase, rightsInheriting, fd, errno})
	return fd, errno
}

func (o *Observer) SockBind(ctx context.Context, fd FD, addr SocketAddress) Errno {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	errno := se.SockBind(ctx, fd, addr)
	o.observe(ctx, &SockBindSyscall{fd, addr, errno})
	return errno
}

func (o *Observer) SockConnect(ctx context.Context, fd FD, addr SocketAddress) Errno {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	errno := se.SockConnect(ctx, fd, addr)
	o.observe(ctx, &SockConnectSyscall{fd, addr, errno})
	return errno
}

func (o *Observer) SockListen(ctx context.Context, fd FD, backlog int) Errno {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	errno := se.SockListen(ctx, fd, backlog)
	o.observe(ctx, &SockListenSyscall{fd, backlog, errno})
	return errno
}

func (o *Observer) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress) (Size, Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return 0, ENOSYS
	}
	size, errno := se.SockSendTo(ctx, fd, iovecs, iflags, addr)
	o.observe(ctx, &SockSendToSyscall{fd, iovecs, iflags, addr, size, errno})
	return size, errno
}

func (o *Observer) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, SocketAddress, Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return 0, 0, nil, ENOSYS
	}
	size, oflags, addr, errno := se.SockRecvFrom(ctx, fd, iovecs, iflags)
	o.observe(ctx, &SockRecvFromSyscall{fd, iovecs, iflags, size, oflags, addr, errno})
	return size, oflags, addr, errno
}

func (o *Observer) SockGetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (int, Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return 0, ENOSYS
	}
	value, errno := se.SockGetOptInt(ctx, fd, level, option)
	o.observe(ctx, &SockGetOptIntSyscall{fd, level, option, value, errno})
	return value, errno
}

func (o *Observer) SockSetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value int) Errno {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	errno := se.SockSetOptInt(ctx, fd, level, option, value)
	o.observe(ctx, &SockSetOptIntSyscall{fd, level, option, value, errno})
	return errno
}

func (o *Observer) SockLocalAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	addr, errno := se.SockLocalAddress(ctx, fd)
	o.observe(ctx, &SockLocalAddressSyscall{fd, addr, errno})
	return addr, errno
}

func (o *Observer) SockPeerAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	se, ok := o.System.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	addr, errno := se.SockPeerAddress(ctx, fd)
	o.observe(ctx, &SockPeerAddressSyscall{fd, addr, errno})
	return addr, errno
}
