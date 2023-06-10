package wasicall

import (
	"context"
	"fmt"

	"github.com/stealthrocket/wasi-go"
)

func syscallString(s Syscall) string {
	return fmt.Sprintf("%s(%v) => %v", s.ID(), s.Params(), s.Results())
}

type SocketsSystem interface {
	wasi.System
	wasi.SocketsExtension
}

// pushParams pushes Syscall params into a wasi.System.
func pushParams(ctx context.Context, syscall Syscall, system SocketsSystem) {
	switch s := syscall.(type) {
	case *ArgsSizesGetSyscall:
		system.ArgsSizesGet(ctx)
	case *ArgsGetSyscall:
		system.ArgsGet(ctx)
	case *EnvironSizesGetSyscall:
		system.EnvironSizesGet(ctx)
	case *EnvironGetSyscall:
		system.EnvironGet(ctx)
	case *ClockResGetSyscall:
		system.ClockResGet(ctx, s.ClockID)
	case *ClockTimeGetSyscall:
		system.ClockTimeGet(ctx, s.ClockID, s.Precision)
	case *FDAdviseSyscall:
		system.FDAdvise(ctx, s.FD, s.Offset, s.Length, s.Advice)
	case *FDAllocateSyscall:
		system.FDAllocate(ctx, s.FD, s.Offset, s.Length)
	case *FDCloseSyscall:
		system.FDClose(ctx, s.FD)
	case *FDDataSyncSyscall:
		system.FDDataSync(ctx, s.FD)
	case *FDStatGetSyscall:
		system.FDStatGet(ctx, s.FD)
	case *FDStatSetFlagsSyscall:
		system.FDStatSetFlags(ctx, s.FD, s.Flags)
	case *FDStatSetRightsSyscall:
		system.FDStatSetRights(ctx, s.FD, s.RightsBase, s.RightsInheriting)
	case *FDFileStatGetSyscall:
		system.FDFileStatGet(ctx, s.FD)
	case *FDFileStatSetSizeSyscall:
		system.FDFileStatSetSize(ctx, s.FD, s.Size)
	case *FDFileStatSetTimesSyscall:
		system.FDFileStatSetTimes(ctx, s.FD, s.AccessTime, s.ModifyTime, s.Flags)
	case *FDPreadSyscall:
		system.FDPread(ctx, s.FD, s.IOVecs, s.Offset)
	case *FDPreStatGetSyscall:
		system.FDPreStatGet(ctx, s.FD)
	case *FDPreStatDirNameSyscall:
		system.FDPreStatDirName(ctx, s.FD)
	case *FDPwriteSyscall:
		system.FDPwrite(ctx, s.FD, s.IOVecs, s.Offset)
	case *FDReadSyscall:
		system.FDRead(ctx, s.FD, s.IOVecs)
	case *FDReadDirSyscall:
		system.FDReadDir(ctx, s.FD, s.Entries, s.Cookie, s.BufferSizeBytes)
	case *FDRenumberSyscall:
		system.FDRenumber(ctx, s.From, s.To)
	case *FDSeekSyscall:
		system.FDSeek(ctx, s.FD, s.Offset, s.Whence)
	case *FDSyncSyscall:
		system.FDSync(ctx, s.FD)
	case *FDTellSyscall:
		system.FDTell(ctx, s.FD)
	case *FDWriteSyscall:
		system.FDWrite(ctx, s.FD, s.IOVecs)
	case *PathCreateDirectorySyscall:
		system.PathCreateDirectory(ctx, s.FD, s.Path)
	case *PathFileStatGetSyscall:
		system.PathFileStatGet(ctx, s.FD, s.LookupFlags, s.Path)
	case *PathFileStatSetTimesSyscall:
		system.PathFileStatSetTimes(ctx, s.FD, s.LookupFlags, s.Path, s.AccessTime, s.ModifyTime, s.Flags)
	case *PathLinkSyscall:
		system.PathLink(ctx, s.OldFD, s.OldFlags, s.OldPath, s.NewFD, s.NewPath)
	case *PathOpenSyscall:
		system.PathOpen(ctx, s.FD, s.DirFlags, s.Path, s.OpenFlags, s.RightsBase, s.RightsInheriting, s.FDFlags)
	case *PathReadLinkSyscall:
		system.PathReadLink(ctx, s.FD, s.Path, s.Output)
	case *PathRemoveDirectorySyscall:
		system.PathRemoveDirectory(ctx, s.FD, s.Path)
	case *PathRenameSyscall:
		system.PathRename(ctx, s.FD, s.OldPath, s.NewFD, s.NewPath)
	case *PathSymlinkSyscall:
		system.PathSymlink(ctx, s.OldPath, s.FD, s.NewPath)
	case *PathUnlinkFileSyscall:
		system.PathUnlinkFile(ctx, s.FD, s.Path)
	case *PollOneOffSyscall:
		system.PollOneOff(ctx, s.Subscriptions, s.Events)
	case *ProcExitSyscall:
		system.ProcExit(ctx, s.ExitCode)
	case *ProcRaiseSyscall:
		system.ProcRaise(ctx, s.Signal)
	case *SchedYieldSyscall:
		system.SchedYield(ctx)
	case *RandomGetSyscall:
		system.RandomGet(ctx, s.B)
	case *SockAcceptSyscall:
		system.SockAccept(ctx, s.FD, s.Flags)
	case *SockShutdownSyscall:
		system.SockShutdown(ctx, s.FD, s.Flags)
	case *SockRecvSyscall:
		system.SockRecv(ctx, s.FD, s.IOVecs, s.IFlags)
	case *SockSendSyscall:
		system.SockSend(ctx, s.FD, s.IOVecs, s.IFlags)
	case *SockOpenSyscall:
		system.SockOpen(ctx, s.Family, s.SocketType, s.Protocol, s.RightsBase, s.RightsInheriting)
	case *SockBindSyscall:
		system.SockBind(ctx, s.FD, s.Addr)
	case *SockConnectSyscall:
		system.SockConnect(ctx, s.FD, s.Addr)
	case *SockListenSyscall:
		system.SockListen(ctx, s.FD, s.Backlog)
	case *SockSendToSyscall:
		system.SockSendTo(ctx, s.FD, s.IOVecs, s.IFlags, s.Addr)
	case *SockRecvFromSyscall:
		system.SockRecvFrom(ctx, s.FD, s.IOVecs, s.IFlags)
	case *SockGetOptSyscall:
		system.SockGetOpt(ctx, s.FD, s.Level, s.Option)
	case *SockSetOptSyscall:
		system.SockSetOpt(ctx, s.FD, s.Level, s.Option, s.Value)
	case *SockLocalAddressSyscall:
		system.SockLocalAddress(ctx, s.FD)
	case *SockRemoteAddressSyscall:
		system.SockRemoteAddress(ctx, s.FD)
	case *SockAddressInfoSyscall:
		system.SockAddressInfo(ctx, s.Name, s.Service, s.Hints, s.Res)
	default:
		panic("not implemented")
	}
}

// pullResults is the inverse to pushParams. You can use it to pull
// Syscall results through a wasi.System.
type pullResults struct{ Syscall }

func cast[T Syscall](s Syscall) T {
	t, ok := s.(T)
	if !ok {
		panic("unexpected syscall")
	}
	return t
}

func (p *pullResults) ArgsSizesGet(ctx context.Context) (argCount int, stringBytes int, errno wasi.Errno) {
	s := cast[*ArgsSizesGetSyscall](p.Syscall)
	return s.ArgCount, s.StringBytes, s.Errno
}

func (p *pullResults) ArgsGet(ctx context.Context) ([]string, wasi.Errno) {
	s := cast[*ArgsGetSyscall](p.Syscall)
	return s.Args, s.Errno
}

func (p *pullResults) EnvironSizesGet(ctx context.Context) (argCount int, stringBytes int, errno wasi.Errno) {
	s := cast[*EnvironSizesGetSyscall](p.Syscall)
	return s.EnvCount, s.StringBytes, s.Errno
}

func (p *pullResults) EnvironGet(ctx context.Context) ([]string, wasi.Errno) {
	s := cast[*EnvironGetSyscall](p.Syscall)
	return s.Env, s.Errno
}

func (p *pullResults) ClockResGet(ctx context.Context, id wasi.ClockID) (wasi.Timestamp, wasi.Errno) {
	s := cast[*ClockResGetSyscall](p.Syscall)
	return s.Timestamp, s.Errno
}

func (p *pullResults) ClockTimeGet(ctx context.Context, id wasi.ClockID, precision wasi.Timestamp) (wasi.Timestamp, wasi.Errno) {
	s := cast[*ClockTimeGetSyscall](p.Syscall)
	return s.Timestamp, s.Errno
}

func (p *pullResults) FDAdvise(ctx context.Context, fd wasi.FD, offset wasi.FileSize, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	s := cast[*FDAdviseSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDAllocate(ctx context.Context, fd wasi.FD, offset wasi.FileSize, length wasi.FileSize) wasi.Errno {
	s := cast[*FDAllocateSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDClose(ctx context.Context, fd wasi.FD) wasi.Errno {
	s := cast[*FDCloseSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDDataSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	s := cast[*FDDataSyncSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDStatGet(ctx context.Context, fd wasi.FD) (wasi.FDStat, wasi.Errno) {
	s := cast[*FDStatGetSyscall](p.Syscall)
	return s.Stat, s.Errno
}

func (p *pullResults) FDStatSetFlags(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) wasi.Errno {
	s := cast[*FDStatSetFlagsSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDStatSetRights(ctx context.Context, fd wasi.FD, rightsBase, rightsInheriting wasi.Rights) wasi.Errno {
	s := cast[*FDStatSetRightsSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDFileStatGet(ctx context.Context, fd wasi.FD) (wasi.FileStat, wasi.Errno) {
	s := cast[*FDFileStatGetSyscall](p.Syscall)
	return s.Stat, s.Errno
}

func (p *pullResults) FDFileStatSetSize(ctx context.Context, fd wasi.FD, size wasi.FileSize) wasi.Errno {
	s := cast[*FDFileStatSetSizeSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDFileStatSetTimes(ctx context.Context, fd wasi.FD, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	s := cast[*FDFileStatSetTimesSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDPread(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	s := cast[*FDPreadSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *pullResults) FDPreStatGet(ctx context.Context, fd wasi.FD) (wasi.PreStat, wasi.Errno) {
	s := cast[*FDPreStatGetSyscall](p.Syscall)
	return s.Stat, s.Errno
}

func (p *pullResults) FDPreStatDirName(ctx context.Context, fd wasi.FD) (string, wasi.Errno) {
	s := cast[*FDPreStatDirNameSyscall](p.Syscall)
	return s.Name, s.Errno
}

func (p *pullResults) FDPwrite(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	s := cast[*FDPwriteSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *pullResults) FDRead(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	s := cast[*FDReadSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *pullResults) FDReadDir(ctx context.Context, fd wasi.FD, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	s := cast[*FDReadDirSyscall](p.Syscall)
	return len(s.Entries), s.Errno
}

func (p *pullResults) FDRenumber(ctx context.Context, from, to wasi.FD) wasi.Errno {
	s := cast[*FDRenumberSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDSeek(ctx context.Context, fd wasi.FD, offset wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	s := cast[*FDSeekSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *pullResults) FDSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	s := cast[*FDSyncSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) FDTell(ctx context.Context, fd wasi.FD) (wasi.FileSize, wasi.Errno) {
	s := cast[*FDTellSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *pullResults) FDWrite(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	s := cast[*FDWriteSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *pullResults) PathCreateDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	s := cast[*PathCreateDirectorySyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) PathFileStatGet(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	s := cast[*PathFileStatGetSyscall](p.Syscall)
	return s.Stat, s.Errno
}

func (p *pullResults) PathFileStatSetTimes(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	s := cast[*PathFileStatSetTimesSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) PathLink(ctx context.Context, oldFD wasi.FD, oldFlags wasi.LookupFlags, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	s := cast[*PathLinkSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) PathOpen(ctx context.Context, fd wasi.FD, dirFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (wasi.FD, wasi.Errno) {
	s := cast[*PathOpenSyscall](p.Syscall)
	return s.NewFD, s.Errno
}

func (p *pullResults) PathReadLink(ctx context.Context, fd wasi.FD, path string, buffer []byte) (int, wasi.Errno) {
	s := cast[*PathReadLinkSyscall](p.Syscall)
	return len(s.Output), s.Errno
}

func (p *pullResults) PathRemoveDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	s := cast[*PathRemoveDirectorySyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) PathRename(ctx context.Context, fd wasi.FD, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	s := cast[*PathRenameSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) PathSymlink(ctx context.Context, oldPath string, fd wasi.FD, newPath string) wasi.Errno {
	s := cast[*PathSymlinkSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) PathUnlinkFile(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	s := cast[*PathUnlinkFileSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	s := cast[*PollOneOffSyscall](p.Syscall)
	return len(s.Events), s.Errno
}

func (p *pullResults) ProcExit(ctx context.Context, exitCode wasi.ExitCode) wasi.Errno {
	s := cast[*ProcExitSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) ProcRaise(ctx context.Context, signal wasi.Signal) wasi.Errno {
	s := cast[*ProcRaiseSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) SchedYield(ctx context.Context) wasi.Errno {
	s := cast[*SchedYieldSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) RandomGet(ctx context.Context, b []byte) wasi.Errno {
	s := cast[*RandomGetSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) SockAccept(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) (wasi.FD, wasi.SocketAddress, wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockAcceptSyscall](p.Syscall)
	return s.NewFD, s.Peer, s.Addr, s.Errno
}

func (p *pullResults) SockRecv(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	s := cast[*SockRecvSyscall](p.Syscall)
	return s.Size, s.OFlags, s.Errno
}

func (p *pullResults) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	s := cast[*SockSendSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *pullResults) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	s := cast[*SockShutdownSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) Close(ctx context.Context) error {
	return nil
}

func (p *pullResults) SockOpen(ctx context.Context, family wasi.ProtocolFamily, socketType wasi.SocketType, protocol wasi.Protocol, rightsBase, rightsInheriting wasi.Rights) (wasi.FD, wasi.Errno) {
	s := cast[*SockOpenSyscall](p.Syscall)
	return s.FD, s.Errno
}

func (p *pullResults) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockBindSyscall](p.Syscall)
	return s.Addr, s.Errno
}

func (p *pullResults) SockConnect(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockConnectSyscall](p.Syscall)
	return s.Addr, s.Errno
}

func (p *pullResults) SockListen(ctx context.Context, fd wasi.FD, backlog int) wasi.Errno {
	s := cast[*SockListenSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) SockSendTo(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	s := cast[*SockSendToSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *pullResults) SockRecvFrom(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockRecvFromSyscall](p.Syscall)
	return s.Size, s.OFlags, s.Addr, s.Errno
}

func (p *pullResults) SockGetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	s := cast[*SockGetOptSyscall](p.Syscall)
	return s.Value, s.Errno
}

func (p *pullResults) SockSetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	s := cast[*SockSetOptSyscall](p.Syscall)
	return s.Errno
}

func (p *pullResults) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockLocalAddressSyscall](p.Syscall)
	return s.Addr, s.Errno
}

func (p *pullResults) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockRemoteAddressSyscall](p.Syscall)
	return s.Addr, s.Errno
}

func (p *pullResults) SockAddressInfo(ctx context.Context, name, service string, hints wasi.AddressInfo, results []wasi.AddressInfo) (int, wasi.Errno) {
	s := cast[*SockAddressInfoSyscall](p.Syscall)
	return len(s.Res), s.Errno
}

var _ wasi.System = (*pullResults)(nil)
var _ wasi.SocketsExtension = (*pullResults)(nil)

type errnoSystem wasi.Errno

func (e errnoSystem) ArgsSizesGet(ctx context.Context) (argCount int, stringBytes int, errno wasi.Errno) {
	return 0, 0, wasi.Errno(e)
}

func (e errnoSystem) ArgsGet(ctx context.Context) ([]string, wasi.Errno) {
	return nil, wasi.Errno(e)
}

func (e errnoSystem) EnvironSizesGet(ctx context.Context) (argCount int, stringBytes int, errno wasi.Errno) {
	return 0, 0, wasi.Errno(e)
}

func (e errnoSystem) EnvironGet(ctx context.Context) ([]string, wasi.Errno) {
	return nil, wasi.Errno(e)
}

func (e errnoSystem) ClockResGet(ctx context.Context, id wasi.ClockID) (wasi.Timestamp, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) ClockTimeGet(ctx context.Context, id wasi.ClockID, precision wasi.Timestamp) (wasi.Timestamp, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) FDAdvise(ctx context.Context, fd wasi.FD, offset wasi.FileSize, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDAllocate(ctx context.Context, fd wasi.FD, offset wasi.FileSize, length wasi.FileSize) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDClose(ctx context.Context, fd wasi.FD) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDDataSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDStatGet(ctx context.Context, fd wasi.FD) (wasi.FDStat, wasi.Errno) {
	return wasi.FDStat{}, wasi.Errno(e)
}

func (e errnoSystem) FDStatSetFlags(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDStatSetRights(ctx context.Context, fd wasi.FD, rightsBase, rightsInheriting wasi.Rights) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDFileStatGet(ctx context.Context, fd wasi.FD) (wasi.FileStat, wasi.Errno) {
	return wasi.FileStat{}, wasi.Errno(e)
}

func (e errnoSystem) FDFileStatSetSize(ctx context.Context, fd wasi.FD, size wasi.FileSize) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDFileStatSetTimes(ctx context.Context, fd wasi.FD, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDPread(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) FDPreStatGet(ctx context.Context, fd wasi.FD) (wasi.PreStat, wasi.Errno) {
	return wasi.PreStat{}, wasi.Errno(e)
}

func (e errnoSystem) FDPreStatDirName(ctx context.Context, fd wasi.FD) (string, wasi.Errno) {
	return "", wasi.Errno(e)
}

func (e errnoSystem) FDPwrite(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) FDRead(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) FDReadDir(ctx context.Context, fd wasi.FD, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) FDRenumber(ctx context.Context, from, to wasi.FD) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDSeek(ctx context.Context, fd wasi.FD, offset wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) FDSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) FDTell(ctx context.Context, fd wasi.FD) (wasi.FileSize, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) FDWrite(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) PathCreateDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) PathFileStatGet(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return wasi.FileStat{}, wasi.Errno(e)
}

func (e errnoSystem) PathFileStatSetTimes(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) PathLink(ctx context.Context, oldFD wasi.FD, oldFlags wasi.LookupFlags, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) PathOpen(ctx context.Context, fd wasi.FD, dirFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (wasi.FD, wasi.Errno) {
	return -1, wasi.Errno(e)
}

func (e errnoSystem) PathReadLink(ctx context.Context, fd wasi.FD, path string, buffer []byte) (int, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) PathRemoveDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) PathRename(ctx context.Context, fd wasi.FD, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) PathSymlink(ctx context.Context, oldPath string, fd wasi.FD, newPath string) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) PathUnlinkFile(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) ProcExit(ctx context.Context, exitCode wasi.ExitCode) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) ProcRaise(ctx context.Context, signal wasi.Signal) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) SchedYield(ctx context.Context) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) RandomGet(ctx context.Context, b []byte) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) SockAccept(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) (wasi.FD, wasi.SocketAddress, wasi.SocketAddress, wasi.Errno) {
	return -1, nil, nil, wasi.Errno(e)
}

func (e errnoSystem) SockRecv(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	return 0, 0, wasi.Errno(e)
}

func (e errnoSystem) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) Close(ctx context.Context) error {
	return nil
}

func (e errnoSystem) SockOpen(ctx context.Context, family wasi.ProtocolFamily, socketType wasi.SocketType, protocol wasi.Protocol, rightsBase, rightsInheriting wasi.Rights) (wasi.FD, wasi.Errno) {
	return -1, wasi.Errno(e)
}

func (e errnoSystem) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.Errno(e)
}

func (e errnoSystem) SockConnect(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.Errno(e)
}

func (e errnoSystem) SockListen(ctx context.Context, fd wasi.FD, backlog int) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) SockSendTo(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	return 0, wasi.Errno(e)
}

func (e errnoSystem) SockRecvFrom(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	return 0, 0, nil, wasi.Errno(e)
}

func (e errnoSystem) SockGetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	return nil, wasi.Errno(e)
}

func (e errnoSystem) SockSetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	return wasi.Errno(e)
}

func (e errnoSystem) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.Errno(e)
}

func (e errnoSystem) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.Errno(e)
}

func (e errnoSystem) SockAddressInfo(ctx context.Context, name, service string, hints wasi.AddressInfo, results []wasi.AddressInfo) (int, wasi.Errno) {
	return 0, wasi.Errno(e)
}

var _ wasi.System = (*errnoSystem)(nil)
var _ wasi.SocketsExtension = (*errnoSystem)(nil)
