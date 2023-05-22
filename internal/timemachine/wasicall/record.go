package wasicall

import (
	"context"
	"time"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	. "github.com/stealthrocket/wasi-go"
)

// Recorder wraps a wasi.System to record system calls.
type Recorder struct {
	system    System
	startTime time.Time
	write     func(*timemachine.RecordBuilder)

	encoder Encoder
	builder timemachine.RecordBuilder
}

var _ System = (*Recorder)(nil)
var _ SocketsExtension = (*Recorder)(nil)

// NewRecorder creates a new Recorder.
//
// The provided write function must consume the record immediately, as it's
// reused across function calls.
func NewRecorder(system System, startTime time.Time, write func(*timemachine.RecordBuilder)) *Recorder {
	return &Recorder{
		system:    system,
		startTime: startTime,
		write:     write,
	}
}

func (r *Recorder) record(s Syscall, b []byte) {
	r.builder.Reset(r.startTime)
	r.builder.SetTimestamp(time.Now())
	r.builder.SetFunctionID(int(s))
	r.builder.SetFunctionCall(b)
	r.write(&r.builder)
}

func (r *Recorder) Preopen(hostfd int, path string, fdstat FDStat) FD {
	// Preopen is not currently recorded.
	return r.system.Preopen(hostfd, path, fdstat)
}

func (r *Recorder) Register(hostfd int, fdstat FDStat) FD {
	// Register is not currently recorded.
	return r.system.Register(hostfd, fdstat)
}

func (r *Recorder) ArgsGet(ctx context.Context) ([]string, Errno) {
	args, errno := r.system.ArgsGet(ctx)
	r.record(ArgsGet, r.encoder.ArgsGet(args, errno))
	return args, errno
}

func (r *Recorder) EnvironGet(ctx context.Context) ([]string, Errno) {
	env, errno := r.system.EnvironGet(ctx)
	r.record(EnvironGet, r.encoder.EnvironGet(env, errno))
	return env, errno
}

func (r *Recorder) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	timestamp, errno := r.system.ClockResGet(ctx, id)
	r.record(ClockResGet, r.encoder.ClockResGet(id, timestamp, errno))
	return timestamp, errno
}

func (r *Recorder) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	timestamp, errno := r.system.ClockTimeGet(ctx, id, precision)
	r.record(ClockTimeGet, r.encoder.ClockTimeGet(id, precision, timestamp, errno))
	return timestamp, errno
}

func (r *Recorder) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	errno := r.system.FDAdvise(ctx, fd, offset, length, advice)
	r.record(FDAdvise, r.encoder.FDAdvise(fd, offset, length, advice, errno))
	return errno
}

func (r *Recorder) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	errno := r.system.FDAllocate(ctx, fd, offset, length)
	r.record(FDAllocate, r.encoder.FDAllocate(fd, offset, length, errno))
	return errno
}

func (r *Recorder) FDClose(ctx context.Context, fd FD) Errno {
	errno := r.system.FDClose(ctx, fd)
	r.record(FDClose, r.encoder.FDClose(fd, errno))
	return errno
}

func (r *Recorder) FDDataSync(ctx context.Context, fd FD) Errno {
	errno := r.system.FDDataSync(ctx, fd)
	r.record(FDDataSync, r.encoder.FDDataSync(fd, errno))
	return errno
}

func (r *Recorder) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	stat, errno := r.system.FDStatGet(ctx, fd)
	r.record(FDStatGet, r.encoder.FDStatGet(fd, stat, errno))
	return stat, errno
}

func (r *Recorder) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	errno := r.system.FDStatSetFlags(ctx, fd, flags)
	r.record(FDStatSetFlags, r.encoder.FDStatSetFlags(fd, flags, errno))
	return errno
}

func (r *Recorder) FDStatSetRights(ctx context.Context, fd FD, rightsBase, rightsInheriting Rights) Errno {
	errno := r.system.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
	r.record(FDStatSetRights, r.encoder.FDStatSetRights(fd, rightsBase, rightsInheriting, errno))
	return errno
}

func (r *Recorder) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	stat, errno := r.system.FDFileStatGet(ctx, fd)
	r.record(FDFileStatGet, r.encoder.FDFileStatGet(fd, stat, errno))
	return stat, errno
}

func (r *Recorder) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	errno := r.system.FDFileStatSetSize(ctx, fd, size)
	r.record(FDFileStatSetSize, r.encoder.FDFileStatSetSize(fd, size, errno))
	return errno
}

func (r *Recorder) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := r.system.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	r.record(FDFileStatSetTimes, r.encoder.FDFileStatSetTimes(fd, accessTime, modifyTime, flags, errno))
	return errno
}

func (r *Recorder) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	size, errno := r.system.FDPread(ctx, fd, iovecs, offset)
	r.record(FDPread, r.encoder.FDPread(fd, iovecs, offset, size, errno))
	return size, errno
}

func (r *Recorder) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	stat, errno := r.system.FDPreStatGet(ctx, fd)
	r.record(FDPreStatGet, r.encoder.FDPreStatGet(fd, stat, errno))
	return stat, errno
}

func (r *Recorder) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	name, errno := r.system.FDPreStatDirName(ctx, fd)
	r.record(FDPreStatDirName, r.encoder.FDPreStatDirName(fd, name, errno))
	return name, errno
}

func (r *Recorder) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	n, errno := r.system.FDPwrite(ctx, fd, iovecs, offset)
	r.record(FDPwrite, r.encoder.FDPwrite(fd, iovecs, offset, n, errno))
	return n, errno
}

func (r *Recorder) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	n, errno := r.system.FDRead(ctx, fd, iovecs)
	r.record(FDRead, r.encoder.FDRead(fd, iovecs, n, errno))
	return n, errno
}

func (r *Recorder) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	n, errno := r.system.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	r.record(FDReadDir, r.encoder.FDReadDir(fd, entries, cookie, bufferSizeBytes, n, errno))
	return n, errno
}

func (r *Recorder) FDRenumber(ctx context.Context, from, to FD) Errno {
	errno := r.system.FDRenumber(ctx, from, to)
	r.record(FDRenumber, r.encoder.FDRenumber(from, to, errno))
	return errno
}

func (r *Recorder) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	result, errno := r.system.FDSeek(ctx, fd, offset, whence)
	r.record(FDSeek, r.encoder.FDSeek(fd, offset, whence, result, errno))
	return result, errno
}

func (r *Recorder) FDSync(ctx context.Context, fd FD) Errno {
	errno := r.system.FDSync(ctx, fd)
	r.record(FDSync, r.encoder.FDSync(fd, errno))
	return errno
}

func (r *Recorder) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	fileSize, errno := r.system.FDTell(ctx, fd)
	r.record(FDTell, r.encoder.FDTell(fd, fileSize, errno))
	return fileSize, errno
}

func (r *Recorder) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	n, errno := r.system.FDWrite(ctx, fd, iovecs)
	r.record(FDWrite, r.encoder.FDWrite(fd, iovecs, n, errno))
	return n, errno
}

func (r *Recorder) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := r.system.PathCreateDirectory(ctx, fd, path)
	r.record(PathCreateDirectory, r.encoder.PathCreateDirectory(fd, path, errno))
	return errno
}

func (r *Recorder) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	filestat, errno := r.system.PathFileStatGet(ctx, fd, lookupFlags, path)
	r.record(PathFileStatGet, r.encoder.PathFileStatGet(fd, lookupFlags, path, filestat, errno))
	return filestat, errno
}

func (r *Recorder) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	errno := r.system.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	r.record(PathFileStatSetTimes, r.encoder.PathFileStatSetTimes(fd, lookupFlags, path, accessTime, modifyTime, flags, errno))
	return errno
}

func (r *Recorder) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	errno := r.system.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	r.record(PathLink, r.encoder.PathLink(oldFD, oldFlags, oldPath, newFD, newPath, errno))
	return errno
}

func (r *Recorder) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	newfd, errno := r.system.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	r.record(PathOpen, r.encoder.PathOpen(fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno))
	return fd, errno
}

func (r *Recorder) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) ([]byte, Errno) {
	result, errno := r.system.PathReadLink(ctx, fd, path, buffer)
	r.record(PathReadLink, r.encoder.PathReadLink(fd, path, buffer, result, errno))
	return result, errno
}

func (r *Recorder) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	errno := r.system.PathRemoveDirectory(ctx, fd, path)
	r.record(PathRemoveDirectory, r.encoder.PathRemoveDirectory(fd, path, errno))
	return errno
}

func (r *Recorder) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	errno := r.system.PathRename(ctx, fd, oldPath, newFD, newPath)
	r.record(PathRename, r.encoder.PathRename(fd, oldPath, newFD, newPath, errno))
	return errno
}

func (r *Recorder) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	errno := r.system.PathSymlink(ctx, oldPath, fd, newPath)
	r.record(PathSymlink, r.encoder.PathSymlink(oldPath, fd, newPath, errno))
	return errno
}

func (r *Recorder) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	errno := r.system.PathUnlinkFile(ctx, fd, path)
	r.record(PathUnlinkFile, r.encoder.PathUnlinkFile(fd, path, errno))
	return errno
}

func (r *Recorder) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	n, errno := r.system.PollOneOff(ctx, subscriptions, events)
	r.record(PollOneOff, r.encoder.PollOneOff(subscriptions, events, n, errno))
	return n, errno
}

func (r *Recorder) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	// For ProcExit, we record the entry before the call in case it
	// panics with sys.ExitError or calls os.Exit.
	r.record(ProcExit, r.encoder.ProcExit(exitCode, ESUCCESS))
	errno := r.system.ProcExit(ctx, exitCode)
	return errno
}

func (r *Recorder) ProcRaise(ctx context.Context, signal Signal) Errno {
	errno := r.system.ProcRaise(ctx, signal)
	r.record(ProcRaise, r.encoder.ProcRaise(signal, errno))
	return errno
}

func (r *Recorder) SchedYield(ctx context.Context) Errno {
	errno := r.system.SchedYield(ctx)
	r.record(SchedYield, r.encoder.SchedYield(errno))
	return errno
}

func (r *Recorder) RandomGet(ctx context.Context, b []byte) Errno {
	errno := r.system.RandomGet(ctx, b)
	r.record(RandomGet, r.encoder.RandomGet(b, errno))
	return errno
}

func (r *Recorder) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, Errno) {
	newfd, errno := r.system.SockAccept(ctx, fd, flags)
	r.record(SockAccept, r.encoder.SockAccept(fd, flags, newfd, errno))
	return newfd, errno
}

func (r *Recorder) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	errno := r.system.SockShutdown(ctx, fd, flags)
	r.record(SockShutdown, r.encoder.SockShutdown(fd, flags, errno))
	return errno
}

func (r *Recorder) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, Errno) {
	n, oflags, errno := r.system.SockRecv(ctx, fd, iovecs, iflags)
	r.record(SockRecv, r.encoder.SockRecv(fd, iovecs, iflags, n, oflags, errno))
	return n, oflags, errno
}

func (r *Recorder) SockSend(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags) (Size, Errno) {
	n, errno := r.system.SockSend(ctx, fd, iovecs, iflags)
	r.record(SockSend, r.encoder.SockSend(fd, iovecs, iflags, n, errno))
	return n, errno
}

func (r *Recorder) SockOpen(ctx context.Context, pf ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights) (FD, Errno) {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return -1, ENOSYS
	}
	fd, errno := s.SockOpen(ctx, pf, socketType, protocol, rightsBase, rightsInheriting)
	r.record(SockOpen, r.encoder.SockOpen(pf, socketType, protocol, rightsBase, rightsInheriting, fd, errno))
	return fd, errno
}

func (r *Recorder) SockBind(ctx context.Context, fd FD, addr SocketAddress) Errno {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	errno := s.SockBind(ctx, fd, addr)
	r.record(SockBind, r.encoder.SockBind(fd, addr, errno))
	return errno
}

func (r *Recorder) SockConnect(ctx context.Context, fd FD, addr SocketAddress) Errno {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	errno := s.SockConnect(ctx, fd, addr)
	r.record(SockConnect, r.encoder.SockConnect(fd, addr, errno))
	return errno
}

func (r *Recorder) SockListen(ctx context.Context, fd FD, backlog int) Errno {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	errno := s.SockListen(ctx, fd, backlog)
	r.record(SockListen, r.encoder.SockListen(fd, backlog, errno))
	return errno
}

func (r *Recorder) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress) (Size, Errno) {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return 0, ENOSYS
	}
	n, errno := s.SockSendTo(ctx, fd, iovecs, iflags, addr)
	r.record(SockSendTo, r.encoder.SockSendTo(fd, iovecs, iflags, addr, n, errno))
	return n, errno
}

func (r *Recorder) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, SocketAddress, Errno) {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return 0, 0, nil, ENOSYS
	}
	n, oflags, addr, errno := s.SockRecvFrom(ctx, fd, iovecs, iflags)
	r.record(SockRecvFrom, r.encoder.SockRecvFrom(fd, iovecs, iflags, n, oflags, addr, errno))
	return n, oflags, addr, errno
}

func (r *Recorder) SockGetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (int, Errno) {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return 0, ENOSYS
	}
	value, errno := s.SockGetOptInt(ctx, fd, level, option)
	r.record(SockGetOptInt, r.encoder.SockGetOptInt(fd, level, option, value, errno))
	return value, errno
}

func (r *Recorder) SockSetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value int) Errno {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return ENOSYS
	}
	errno := s.SockSetOptInt(ctx, fd, level, option, value)
	r.record(SockSetOptInt, r.encoder.SockSetOptInt(fd, level, option, value, errno))
	return errno
}

func (r *Recorder) SockLocalAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	addr, errno := s.SockLocalAddress(ctx, fd)
	r.record(SockLocalAddress, r.encoder.SockLocalAddress(fd, addr, errno))
	return addr, errno
}

func (r *Recorder) SockPeerAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	s, ok := r.system.(SocketsExtension)
	if !ok {
		return nil, ENOSYS
	}
	addr, errno := s.SockPeerAddress(ctx, fd)
	r.record(SockPeerAddress, r.encoder.SockPeerAddress(fd, addr, errno))
	return addr, errno
}

func (r *Recorder) Close(ctx context.Context) error {
	return r.system.Close(ctx)
}
