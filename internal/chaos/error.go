package chaos

import (
	"context"

	"github.com/stealthrocket/wasi-go"
)

// Error wraps the base system to return one that errors on almost all method
// calls.
//
// The error returned may vary depending on the method, most will return EIO,
// ENOBUFS, or ECONNRESET.
//
// Invocations of methods always first call the corresponding method of the base
// system to so that argument validation is performed before generating errors.
// This is done so that we don't produce impossible errors such as EIO on an
// invalid file descriptor number. Note that it means the program cannot assume
// that an operation succeeded or failed when it gets an error from this system,
// which is excellent to exercise recovery mechanisms after an error was
// injected on a write operation.
//
// Errors are not injected in methods that are necessary for a minimal working
// application such as reading arguments or exiting the program. Injecting
// errors in those method calls would likely result in uninteresting conditions,
// like the application not starting at all.
func Error(base wasi.System) wasi.System {
	return &errorSystem{base: base}
}

type errorSystem struct {
	base wasi.System
}

func (s *errorSystem) ArgsSizesGet(ctx context.Context) (int, int, wasi.Errno) {
	return s.base.ArgsSizesGet(ctx)
}

func (s *errorSystem) ArgsGet(ctx context.Context) ([]string, wasi.Errno) {
	return s.base.ArgsGet(ctx)
}

func (s *errorSystem) EnvironSizesGet(ctx context.Context) (int, int, wasi.Errno) {
	return s.base.EnvironSizesGet(ctx)
}

func (s *errorSystem) EnvironGet(ctx context.Context) ([]string, wasi.Errno) {
	return s.base.EnvironGet(ctx)
}

func (s *errorSystem) ClockResGet(ctx context.Context, id wasi.ClockID) (wasi.Timestamp, wasi.Errno) {
	return s.base.ClockResGet(ctx, id)
}

func (s *errorSystem) ClockTimeGet(ctx context.Context, id wasi.ClockID, precision wasi.Timestamp) (wasi.Timestamp, wasi.Errno) {
	return s.base.ClockTimeGet(ctx, id, precision)
}

func (s *errorSystem) FDAdvise(ctx context.Context, fd wasi.FD, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	errno := s.base.FDAdvise(ctx, fd, offset, length, advice)
	return replaceWithEIO(errno)
}

func (s *errorSystem) FDAllocate(ctx context.Context, fd wasi.FD, offset, length wasi.FileSize) wasi.Errno {
	errno := s.base.FDAllocate(ctx, fd, offset, length)
	return replaceWithEIO(errno)
}

func (s *errorSystem) FDClose(ctx context.Context, fd wasi.FD) wasi.Errno {
	errno := s.base.FDClose(ctx, fd)
	return replaceWithEIO(errno)
}

func (s *errorSystem) FDDataSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	errno := s.base.FDDataSync(ctx, fd)
	return replaceWithEIO(errno)
}

func (s *errorSystem) FDStatGet(ctx context.Context, fd wasi.FD) (wasi.FDStat, wasi.Errno) {
	_, errno := s.base.FDStatGet(ctx, fd)
	return wasi.FDStat{}, replaceWithEIO(errno)
}

func (s *errorSystem) FDStatSetFlags(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) wasi.Errno {
	return s.base.FDStatSetFlags(ctx, fd, flags)
}

func (s *errorSystem) FDStatSetRights(ctx context.Context, fd wasi.FD, rightsBase, rightsInheriting wasi.Rights) wasi.Errno {
	return s.base.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
}

func (s *errorSystem) FDFileStatGet(ctx context.Context, fd wasi.FD) (wasi.FileStat, wasi.Errno) {
	_, errno := s.base.FDFileStatGet(ctx, fd)
	return wasi.FileStat{}, replaceWithEIO(errno)
}

func (s *errorSystem) FDFileStatSetSize(ctx context.Context, fd wasi.FD, size wasi.FileSize) wasi.Errno {
	errno := s.base.FDFileStatSetSize(ctx, fd, size)
	return replaceWithEIO(errno)
}

func (s *errorSystem) FDFileStatSetTimes(ctx context.Context, fd wasi.FD, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	errno := s.base.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
	return replaceWithEIO(errno)
}

func (s *errorSystem) FDPread(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	_, errno := s.base.FDPread(ctx, fd, iovs, offset)
	return ^wasi.Size(0), replaceWithEIO(errno)
}

func (s *errorSystem) FDPreStatGet(ctx context.Context, fd wasi.FD) (wasi.PreStat, wasi.Errno) {
	_, errno := s.base.FDPreStatGet(ctx, fd)
	return wasi.PreStat{}, replaceWithEIO(errno)
}

func (s *errorSystem) FDPreStatDirName(ctx context.Context, fd wasi.FD) (string, wasi.Errno) {
	_, errno := s.base.FDPreStatDirName(ctx, fd)
	return "", replaceWithEIO(errno)
}

func (s *errorSystem) FDPwrite(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	_, errno := s.base.FDPwrite(ctx, fd, iovs, offset)
	return ^wasi.Size(0), replaceWithEIO(errno)
}

func (s *errorSystem) FDRead(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	_, errno := s.base.FDRead(ctx, fd, iovs)
	return ^wasi.Size(0), replaceWithEIO(errno)
}

func (s *errorSystem) FDReadDir(ctx context.Context, fd wasi.FD, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	_, errno := s.base.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
	return 0, replaceWithEIO(errno)
}

func (s *errorSystem) FDRenumber(ctx context.Context, from, to wasi.FD) wasi.Errno {
	return s.base.FDRenumber(ctx, from, to)
}

func (s *errorSystem) FDSeek(ctx context.Context, fd wasi.FD, offset wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	_, errno := s.base.FDSeek(ctx, fd, offset, whence)
	return ^wasi.FileSize(0), replaceWithEIO(errno)
}

func (s *errorSystem) FDSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	errno := s.base.FDSync(ctx, fd)
	return replaceWithEIO(errno)
}

func (s *errorSystem) FDTell(ctx context.Context, fd wasi.FD) (wasi.FileSize, wasi.Errno) {
	_, errno := s.base.FDTell(ctx, fd)
	return ^wasi.FileSize(0), replaceWithEIO(errno)
}

func (s *errorSystem) FDWrite(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	_, errno := s.base.FDWrite(ctx, fd, iovs)
	return ^wasi.Size(0), replaceWithEIO(errno)
}

func (s *errorSystem) PathCreateDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	errno := s.base.PathCreateDirectory(ctx, fd, path)
	return replaceWithEIO(errno)
}

func (s *errorSystem) PathFileStatGet(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	_, errno := s.base.PathFileStatGet(ctx, fd, lookupFlags, path)
	return wasi.FileStat{}, replaceWithEIO(errno)
}

func (s *errorSystem) PathFileStatSetTimes(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	errno := s.base.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
	return replaceWithEIO(errno)
}

func (s *errorSystem) PathLink(ctx context.Context, oldFD wasi.FD, oldFlags wasi.LookupFlags, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	errno := s.base.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
	return replaceWithEIO(errno)
}

func (s *errorSystem) PathOpen(ctx context.Context, fd wasi.FD, dirFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (wasi.FD, wasi.Errno) {
	_, errno := s.base.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if errno == wasi.ESUCCESS {
		s.base.FDClose(ctx, fd)
		errno = wasi.EIO
	}
	return -1, errno
}

func (s *errorSystem) PathReadLink(ctx context.Context, fd wasi.FD, path string, buffer []byte) (int, wasi.Errno) {
	_, errno := s.base.PathReadLink(ctx, fd, path, buffer)
	return 0, replaceWithEIO(errno)
}

func (s *errorSystem) PathRemoveDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	errno := s.base.PathRemoveDirectory(ctx, fd, path)
	return replaceWithEIO(errno)
}

func (s *errorSystem) PathRename(ctx context.Context, fd wasi.FD, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	errno := s.base.PathRename(ctx, fd, oldPath, newFD, newPath)
	return replaceWithEIO(errno)
}

func (s *errorSystem) PathSymlink(ctx context.Context, oldPath string, fd wasi.FD, newPath string) wasi.Errno {
	errno := s.base.PathSymlink(ctx, oldPath, fd, newPath)
	return replaceWithEIO(errno)
}

func (s *errorSystem) PathUnlinkFile(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	errno := s.base.PathUnlinkFile(ctx, fd, path)
	return replaceWithEIO(errno)
}

func (s *errorSystem) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	// We don't generate an error from PollOneOff itself because it usually
	// results in crashing the application runtime. Instead, we injected errors
	// on the collected events.
	n, errno := s.base.PollOneOff(ctx, subscriptions, events)
	if n > 0 {
		for i, e := range events[:n] {
			switch e.EventType {
			case wasi.FDReadEvent, wasi.FDWriteEvent:
				events[i].Errno = replaceWithEIO(e.Errno)
			}
		}
	}
	return n, errno
}

func (s *errorSystem) ProcExit(ctx context.Context, exitCode wasi.ExitCode) wasi.Errno {
	return s.base.ProcExit(ctx, exitCode)
}

func (s *errorSystem) ProcRaise(ctx context.Context, signal wasi.Signal) wasi.Errno {
	return s.base.ProcRaise(ctx, signal)
}

func (s *errorSystem) SchedYield(ctx context.Context) wasi.Errno {
	return s.base.SchedYield(ctx)
}

func (s *errorSystem) RandomGet(ctx context.Context, b []byte) wasi.Errno {
	errno := s.base.RandomGet(ctx, b)
	return replaceWithEIO(errno)
}

func (s *errorSystem) SockAccept(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) (wasi.FD, wasi.SocketAddress, wasi.SocketAddress, wasi.Errno) {
	newfd, peer, addr, errno := s.base.SockAccept(ctx, fd, flags)
	if errno == wasi.ESUCCESS {
		// We don't replace the error but instead shutdown the newly accepted
		// socket to make it unusable. This will trigger different types of
		// errors on recv/send even if the error system isn't invoked anymore
		// when interacting with the socket.
		_ = s.base.SockShutdown(ctx, newfd, wasi.ShutdownRD|wasi.ShutdownWR)
	}
	return newfd, peer, addr, errno
}

func (s *errorSystem) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	return s.base.SockShutdown(ctx, fd, flags)
}

func (s *errorSystem) SockRecv(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, iflags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	_, _, errno := s.base.SockRecv(ctx, fd, iovs, iflags)
	return ^wasi.Size(0), wasi.ROFlags(0), replaceWithETIMEDOUT(errno)
}

func (s *errorSystem) SockSend(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, iflags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	_, errno := s.base.SockSend(ctx, fd, iovs, iflags)
	return ^wasi.Size(0), replaceWithETIMEDOUT(errno)
}

func (s *errorSystem) SockOpen(ctx context.Context, pf wasi.ProtocolFamily, socketType wasi.SocketType, protocol wasi.Protocol, rightsBase, rightsInheriting wasi.Rights) (wasi.FD, wasi.Errno) {
	fd, errno := s.base.SockOpen(ctx, pf, socketType, protocol, rightsBase, rightsInheriting)
	if errno == wasi.ESUCCESS {
		s.base.FDClose(ctx, fd)
		errno = wasi.ENOBUFS
	}
	return fd, errno
}

func (s *errorSystem) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	return s.base.SockBind(ctx, fd, addr)
}

func (s *errorSystem) SockConnect(ctx context.Context, fd wasi.FD, peer wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	addr, errno := s.base.SockConnect(ctx, fd, peer)
	switch errno {
	case wasi.ESUCCESS, wasi.EINPROGRESS:
		// Same reasoning as for accepted sockets, shutdown the socket so
		// it becomes unusable, even if the connection was successfully
		// initiated.
		_ = s.base.SockShutdown(ctx, fd, wasi.ShutdownRD|wasi.ShutdownWR)
	}
	return addr, errno
}

func (s *errorSystem) SockListen(ctx context.Context, fd wasi.FD, backlog int) wasi.Errno {
	return s.base.SockListen(ctx, fd, backlog)
}

func (s *errorSystem) SockRecvFrom(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, iflags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	_, _, _, errno := s.base.SockRecvFrom(ctx, fd, iovs, iflags)
	return ^wasi.Size(0), wasi.ROFlags(0), nil, replaceWithENOBUFS(errno)
}

func (s *errorSystem) SockSendTo(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, iflags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	_, errno := s.base.SockSendTo(ctx, fd, iovs, iflags, addr)
	return ^wasi.Size(0), replaceWithENOBUFS(errno)
}

func (s *errorSystem) SockGetOpt(ctx context.Context, fd wasi.FD, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	_, errno := s.base.SockGetOpt(ctx, fd, option)
	return nil, replaceWithENOBUFS(errno)
}

func (s *errorSystem) SockSetOpt(ctx context.Context, fd wasi.FD, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	return s.base.SockSetOpt(ctx, fd, option, value)
}

func (s *errorSystem) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	_, errno := s.base.SockLocalAddress(ctx, fd)
	return nil, replaceWithENOBUFS(errno)
}

func (s *errorSystem) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	_, errno := s.base.SockRemoteAddress(ctx, fd)
	return nil, replaceWithENOBUFS(errno)
}

func (s *errorSystem) SockAddressInfo(ctx context.Context, name, service string, hints wasi.AddressInfo, results []wasi.AddressInfo) (int, wasi.Errno) {
	_, errno := s.base.SockAddressInfo(ctx, name, service, hints, results)
	return 0, replaceWithEIO(errno)
}

func (s *errorSystem) Close(ctx context.Context) error {
	return s.base.Close(ctx)
}

func replaceWithEIO(errno wasi.Errno) wasi.Errno {
	return replaceErrnoWith(errno, wasi.EIO)
}

func replaceWithENOBUFS(errno wasi.Errno) wasi.Errno {
	return replaceErrnoWith(errno, wasi.ENOBUFS)
}

func replaceWithETIMEDOUT(errno wasi.Errno) wasi.Errno {
	return replaceErrnoWith(errno, wasi.ETIMEDOUT)
}

func replaceErrnoWith(errno, replace wasi.Errno) wasi.Errno {
	if errno == wasi.ESUCCESS {
		errno = replace
	}
	return errno
}
