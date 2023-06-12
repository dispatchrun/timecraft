package wasicall

import (
	"context"
	"time"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	. "github.com/stealthrocket/wasi-go"
)

// NewRecorder creates a wasi.System that records system calls.
//
// The provided write function must consume the record immediately as it's
// reused across function calls.
func NewRecorder(system System, startTime time.Time, write func(*timemachine.RecordBuilder)) System {
	return &recorderSystem{
		system:    system,
		startTime: startTime,
		write:     write,
	}
}

type recorderSystem struct {
	system    System
	startTime time.Time
	write     func(*timemachine.RecordBuilder)

	codec   Codec
	builder timemachine.RecordBuilder
	buffer  []byte
}

func (r *recorderSystem) record(s SyscallID, b []byte) {
	r.buffer = b
	r.builder.Reset(r.startTime)
	r.builder.SetTimestamp(time.Now())
	r.builder.SetFunctionID(int(s))
	r.builder.SetFunctionCall(b)
	r.write(&r.builder)
}

func (r *recorderSystem) ArgsSizesGet(ctx context.Context) (int, int, Errno) {
	argCount, stringBytes, errno := r.system.ArgsSizesGet(ctx)
	r.record(ArgsSizesGet, r.codec.EncodeArgsSizesGet(r.buffer[:0], argCount, stringBytes, errno))
	return argCount, stringBytes, errno
}

func (r *recorderSystem) ArgsGet(ctx context.Context) ([]string, Errno) {
	args, errno := r.system.ArgsGet(ctx)
	r.record(ArgsGet, r.codec.EncodeArgsGet(r.buffer[:0], args, errno))
	return args, errno
}

func (r *recorderSystem) EnvironSizesGet(ctx context.Context) (int, int, Errno) {
	envCount, stringBytes, errno := r.system.EnvironSizesGet(ctx)
	r.record(EnvironSizesGet, r.codec.EncodeEnvironSizesGet(r.buffer[:0], envCount, stringBytes, errno))
	return envCount, stringBytes, errno
}

func (r *recorderSystem) EnvironGet(ctx context.Context) ([]string, Errno) {
	env, errno := r.system.EnvironGet(ctx)
	r.record(EnvironGet, r.codec.EncodeEnvironGet(r.buffer[:0], env, errno))
	return env, errno
}

func (r *recorderSystem) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	timestamp, errno := r.system.ClockResGet(ctx, id)
	r.record(ClockResGet, r.codec.EncodeClockResGet(r.buffer[:0], id, timestamp, errno))
	return timestamp, errno
}

func (r *recorderSystem) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	timestamp, errno := r.system.ClockTimeGet(ctx, id, precision)
	r.record(ClockTimeGet, r.codec.EncodeClockTimeGet(r.buffer[:0], id, precision, timestamp, errno))
	return timestamp, errno
}

func (r *recorderSystem) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	errno := r.system.FDAdvise(ctx, fd, offset, length, advice)
	r.record(FDAdvise, r.codec.EncodeFDAdvise(r.buffer[:0], fd, offset, length, advice, errno))
	return errno
}

func (r *recorderSystem) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	errno := r.system.FDAllocate(ctx, fd, offset, length)
	r.record(FDAllocate, r.codec.EncodeFDAllocate(r.buffer[:0], fd, offset, length, errno))
	return errno
}

func (r *recorderSystem) FDClose(ctx context.Context, fd FD) Errno {
	errno := r.system.FDClose(ctx, fd)
	r.record(FDClose, r.codec.EncodeFDClose(r.buffer[:0], fd, errno))
	return errno
}

func (r *recorderSystem) FDDataSync(ctx context.Context, fd FD) Errno {
	errno := r.system.FDDataSync(ctx, fd)
	r.record(FDDataSync, r.codec.EncodeFDDataSync(r.buffer[:0], fd, errno))
	return errno
}

func (r *recorderSystem) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	stat, errno := r.system.FDStatGet(ctx, fd)
	r.record(FDStatGet, r.codec.EncodeFDStatGet(r.buffer[:0], fd, stat, errno))
	return stat, errno
}

func (r *recorderSystem) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	errno := r.system.FDStatSetFlags(ctx, fd, flags)
	r.record(FDStatSetFlags, r.codec.EncodeFDStatSetFlags(r.buffer[:0], fd, flags, errno))
	return errno
}

func (r *recorderSystem) FDStatSetRights(ctx context.Context, fd FD, rightsBase, rightsInheriting Rights) Errno {
	errno := r.system.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	r.record(FDStatSetRights, r.codec.EncodeFDStatSetRights(r.buffer[:0], fd, rightsBase, rightsInheriting, errno))
	return errno
}

func (r *recorderSystem) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	stat, errno := r.system.FDFileStatGet(ctx, fd)
	r.record(FDFileStatGet, r.codec.EncodeFDFileStatGet(r.buffer[:0], fd, stat, errno))
	return stat, errno
}

func (r *recorderSystem) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	errno := r.system.FDFileStatSetSize(ctx, fd, size)
	r.record(FDFileStatSetSize, r.codec.EncodeFDFileStatSetSize(r.buffer[:0], fd, size, errno))
	return errno
}

func (r *recorderSystem) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := r.system.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	r.record(FDFileStatSetTimes, r.codec.EncodeFDFileStatSetTimes(r.buffer[:0], fd, accessTime, modifyTime, flags, errno))
	return errno
}

func (r *recorderSystem) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	size, errno := r.system.FDPread(ctx, fd, iovecs, offset)
	r.record(FDPread, r.codec.EncodeFDPread(r.buffer[:0], fd, iovecs, offset, size, errno))
	return size, errno
}

func (r *recorderSystem) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	stat, errno := r.system.FDPreStatGet(ctx, fd)
	r.record(FDPreStatGet, r.codec.EncodeFDPreStatGet(r.buffer[:0], fd, stat, errno))
	return stat, errno
}

func (r *recorderSystem) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	name, errno := r.system.FDPreStatDirName(ctx, fd)
	r.record(FDPreStatDirName, r.codec.EncodeFDPreStatDirName(r.buffer[:0], fd, name, errno))
	return name, errno
}

func (r *recorderSystem) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	n, errno := r.system.FDPwrite(ctx, fd, iovecs, offset)
	r.record(FDPwrite, r.codec.EncodeFDPwrite(r.buffer[:0], fd, iovecs, offset, n, errno))
	return n, errno
}

func (r *recorderSystem) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	n, errno := r.system.FDRead(ctx, fd, iovecs)
	r.record(FDRead, r.codec.EncodeFDRead(r.buffer[:0], fd, iovecs, n, errno))
	return n, errno
}

func (r *recorderSystem) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	n, errno := r.system.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	if n >= 0 && n <= len(entries) {
		entries = entries[:n]
	} else {
		entries = entries[:0]
	}
	r.record(FDReadDir, r.codec.EncodeFDReadDir(r.buffer[:0], fd, entries, cookie, bufferSizeBytes, errno))
	return n, errno
}

func (r *recorderSystem) FDRenumber(ctx context.Context, from, to FD) Errno {
	errno := r.system.FDRenumber(ctx, from, to)
	r.record(FDRenumber, r.codec.EncodeFDRenumber(r.buffer[:0], from, to, errno))
	return errno
}

func (r *recorderSystem) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	result, errno := r.system.FDSeek(ctx, fd, offset, whence)
	r.record(FDSeek, r.codec.EncodeFDSeek(r.buffer[:0], fd, offset, whence, result, errno))
	return result, errno
}

func (r *recorderSystem) FDSync(ctx context.Context, fd FD) Errno {
	errno := r.system.FDSync(ctx, fd)
	r.record(FDSync, r.codec.EncodeFDSync(r.buffer[:0], fd, errno))
	return errno
}

func (r *recorderSystem) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	fileSize, errno := r.system.FDTell(ctx, fd)
	r.record(FDTell, r.codec.EncodeFDTell(r.buffer[:0], fd, fileSize, errno))
	return fileSize, errno
}

func (r *recorderSystem) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	n, errno := r.system.FDWrite(ctx, fd, iovecs)
	r.record(FDWrite, r.codec.EncodeFDWrite(r.buffer[:0], fd, iovecs, n, errno))
	return n, errno
}

func (r *recorderSystem) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := r.system.PathCreateDirectory(ctx, fd, path)
	r.record(PathCreateDirectory, r.codec.EncodePathCreateDirectory(r.buffer[:0], fd, path, errno))
	return errno
}

func (r *recorderSystem) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	filestat, errno := r.system.PathFileStatGet(ctx, fd, lookupFlags, path)
	r.record(PathFileStatGet, r.codec.EncodePathFileStatGet(r.buffer[:0], fd, lookupFlags, path, filestat, errno))
	return filestat, errno
}

func (r *recorderSystem) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := r.system.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	r.record(PathFileStatSetTimes, r.codec.EncodePathFileStatSetTimes(r.buffer[:0], fd, lookupFlags, path, accessTime, modifyTime, flags, errno))
	return errno
}

func (r *recorderSystem) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	errno := r.system.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	r.record(PathLink, r.codec.EncodePathLink(r.buffer[:0], oldFD, oldFlags, oldPath, newFD, newPath, errno))
	return errno
}

func (r *recorderSystem) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	newfd, errno := r.system.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	r.record(PathOpen, r.codec.EncodePathOpen(r.buffer[:0], fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno))
	return newfd, errno
}

func (r *recorderSystem) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) (int, Errno) {
	n, errno := r.system.PathReadLink(ctx, fd, path, buffer)
	if n >= 0 && n <= len(buffer) {
		buffer = buffer[:n]
	} else {
		buffer = buffer[:0]
	}
	r.record(PathReadLink, r.codec.EncodePathReadLink(r.buffer[:0], fd, path, buffer, errno))
	return n, errno
}

func (r *recorderSystem) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := r.system.PathRemoveDirectory(ctx, fd, path)
	r.record(PathRemoveDirectory, r.codec.EncodePathRemoveDirectory(r.buffer[:0], fd, path, errno))
	return errno
}

func (r *recorderSystem) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	errno := r.system.PathRename(ctx, fd, oldPath, newFD, newPath)
	r.record(PathRename, r.codec.EncodePathRename(r.buffer[:0], fd, oldPath, newFD, newPath, errno))
	return errno
}

func (r *recorderSystem) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	errno := r.system.PathSymlink(ctx, oldPath, fd, newPath)
	r.record(PathSymlink, r.codec.EncodePathSymlink(r.buffer[:0], oldPath, fd, newPath, errno))
	return errno
}

func (r *recorderSystem) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	errno := r.system.PathUnlinkFile(ctx, fd, path)
	r.record(PathUnlinkFile, r.codec.EncodePathUnlinkFile(r.buffer[:0], fd, path, errno))
	return errno
}

func (r *recorderSystem) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	n, errno := r.system.PollOneOff(ctx, subscriptions, events)
	if n >= 0 && n <= len(events) {
		events = events[:n]
	} else {
		events = events[:0]
	}
	r.record(PollOneOff, r.codec.EncodePollOneOff(r.buffer[:0], subscriptions, events, errno))
	return n, errno
}

func (r *recorderSystem) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	// For ProcExit, we record the entry before the call in case it
	// panics with sys.ExitError or calls os.Exit.
	r.record(ProcExit, r.codec.EncodeProcExit(r.buffer[:0], exitCode, ESUCCESS))
	errno := r.system.ProcExit(ctx, exitCode)
	return errno
}

func (r *recorderSystem) ProcRaise(ctx context.Context, signal Signal) Errno {
	errno := r.system.ProcRaise(ctx, signal)
	r.record(ProcRaise, r.codec.EncodeProcRaise(r.buffer[:0], signal, errno))
	return errno
}

func (r *recorderSystem) SchedYield(ctx context.Context) Errno {
	errno := r.system.SchedYield(ctx)
	r.record(SchedYield, r.codec.EncodeSchedYield(r.buffer[:0], errno))
	return errno
}

func (r *recorderSystem) RandomGet(ctx context.Context, b []byte) Errno {
	errno := r.system.RandomGet(ctx, b)
	r.record(RandomGet, r.codec.EncodeRandomGet(r.buffer[:0], b, errno))
	return errno
}

func (r *recorderSystem) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, SocketAddress, SocketAddress, Errno) {
	newfd, peer, addr, errno := r.system.SockAccept(ctx, fd, flags)
	r.record(SockAccept, r.codec.EncodeSockAccept(r.buffer[:0], fd, flags, newfd, peer, addr, errno))
	return newfd, peer, addr, errno
}

func (r *recorderSystem) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	errno := r.system.SockShutdown(ctx, fd, flags)
	r.record(SockShutdown, r.codec.EncodeSockShutdown(r.buffer[:0], fd, flags, errno))
	return errno
}

func (r *recorderSystem) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, Errno) {
	n, oflags, errno := r.system.SockRecv(ctx, fd, iovecs, iflags)
	r.record(SockRecv, r.codec.EncodeSockRecv(r.buffer[:0], fd, iovecs, iflags, n, oflags, errno))
	return n, oflags, errno
}

func (r *recorderSystem) SockSend(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags) (Size, Errno) {
	n, errno := r.system.SockSend(ctx, fd, iovecs, iflags)
	r.record(SockSend, r.codec.EncodeSockSend(r.buffer[:0], fd, iovecs, iflags, n, errno))
	return n, errno
}

func (r *recorderSystem) SockOpen(ctx context.Context, pf ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights) (FD, Errno) {
	fd, errno := r.system.SockOpen(ctx, pf, socketType, protocol, rightsBase, rightsInheriting)
	r.record(SockOpen, r.codec.EncodeSockOpen(r.buffer[:0], pf, socketType, protocol, rightsBase, rightsInheriting, fd, errno))
	return fd, errno
}

func (r *recorderSystem) SockBind(ctx context.Context, fd FD, bind SocketAddress) (SocketAddress, Errno) {
	addr, errno := r.system.SockBind(ctx, fd, bind)
	r.record(SockBind, r.codec.EncodeSockBind(r.buffer[:0], fd, bind, addr, errno))
	return addr, errno
}

func (r *recorderSystem) SockConnect(ctx context.Context, fd FD, peer SocketAddress) (SocketAddress, Errno) {
	addr, errno := r.system.SockConnect(ctx, fd, peer)
	r.record(SockConnect, r.codec.EncodeSockConnect(r.buffer[:0], fd, peer, addr, errno))
	return addr, errno
}

func (r *recorderSystem) SockListen(ctx context.Context, fd FD, backlog int) Errno {
	errno := r.system.SockListen(ctx, fd, backlog)
	r.record(SockListen, r.codec.EncodeSockListen(r.buffer[:0], fd, backlog, errno))
	return errno
}

func (r *recorderSystem) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress) (Size, Errno) {
	n, errno := r.system.SockSendTo(ctx, fd, iovecs, iflags, addr)
	r.record(SockSendTo, r.codec.EncodeSockSendTo(r.buffer[:0], fd, iovecs, iflags, addr, n, errno))
	return n, errno
}

func (r *recorderSystem) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, SocketAddress, Errno) {
	n, oflags, addr, errno := r.system.SockRecvFrom(ctx, fd, iovecs, iflags)
	r.record(SockRecvFrom, r.codec.EncodeSockRecvFrom(r.buffer[:0], fd, iovecs, iflags, n, oflags, addr, errno))
	return n, oflags, addr, errno
}

func (r *recorderSystem) SockGetOpt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (SocketOptionValue, Errno) {
	value, errno := r.system.SockGetOpt(ctx, fd, level, option)
	r.record(SockGetOpt, r.codec.EncodeSockGetOpt(r.buffer[:0], fd, level, option, value, errno))
	return value, errno
}

func (r *recorderSystem) SockSetOpt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value SocketOptionValue) Errno {
	errno := r.system.SockSetOpt(ctx, fd, level, option, value)
	r.record(SockSetOpt, r.codec.EncodeSockSetOpt(r.buffer[:0], fd, level, option, value, errno))
	return errno
}

func (r *recorderSystem) SockLocalAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	addr, errno := r.system.SockLocalAddress(ctx, fd)
	r.record(SockLocalAddress, r.codec.EncodeSockLocalAddress(r.buffer[:0], fd, addr, errno))
	return addr, errno
}

func (r *recorderSystem) SockRemoteAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	addr, errno := r.system.SockRemoteAddress(ctx, fd)
	r.record(SockRemoteAddress, r.codec.EncodeSockRemoteAddress(r.buffer[:0], fd, addr, errno))
	return addr, errno
}

func (r *recorderSystem) SockAddressInfo(ctx context.Context, name, service string, hints AddressInfo, results []AddressInfo) (int, Errno) {
	n, errno := r.system.SockAddressInfo(ctx, name, service, hints, results)
	if n >= 0 && n <= len(results) {
		results = results[:n]
	} else {
		results = results[:0]
	}
	r.record(SockAddressInfo, r.codec.EncodeSockAddressInfo(r.buffer[:0], name, service, hints, results, errno))
	return n, errno
}

func (r *recorderSystem) Close(ctx context.Context) error {
	return r.system.Close(ctx)
}
