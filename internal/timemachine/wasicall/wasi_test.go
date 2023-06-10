package wasicall

import (
	"context"
	"fmt"

	"github.com/stealthrocket/wasi-go"
)

var validSyscalls = []Syscall{}

func syscallString(s Syscall) string {
	return fmt.Sprintf("%s(%v) => %v", s.ID(), s.Params(), s.Results())
}

type SocketsSystem interface {
	wasi.System
	wasi.SocketsExtension
}

// call pushes Syscall params into a wasi.System and returns
// a copy of the Syscall instance that includes the results.
func call(ctx context.Context, system SocketsSystem, syscall Syscall) Syscall {
	switch s := syscall.(type) {
	case *ArgsSizesGetSyscall:
		r := *cast[*ArgsSizesGetSyscall](syscall)
		r.ArgCount, r.StringBytes, r.Errno = system.ArgsSizesGet(ctx)
		return &r
	case *ArgsGetSyscall:
		r := *cast[*ArgsGetSyscall](syscall)
		r.Args, r.Errno = system.ArgsGet(ctx)
		return &r
	case *EnvironSizesGetSyscall:
		r := *cast[*EnvironSizesGetSyscall](syscall)
		r.EnvCount, r.StringBytes, r.Errno = system.EnvironSizesGet(ctx)
		return &r
	case *EnvironGetSyscall:
		r := *cast[*EnvironGetSyscall](syscall)
		r.Env, r.Errno = system.EnvironGet(ctx)
		return &r
	case *ClockResGetSyscall:
		r := *cast[*ClockResGetSyscall](syscall)
		r.Timestamp, r.Errno = system.ClockResGet(ctx, s.ClockID)
		return &r
	case *ClockTimeGetSyscall:
		r := *cast[*ClockTimeGetSyscall](syscall)
		r.Timestamp, r.Errno = system.ClockTimeGet(ctx, s.ClockID, s.Precision)
		return &r
	case *FDAdviseSyscall:
		r := *cast[*FDAdviseSyscall](syscall)
		r.Errno = system.FDAdvise(ctx, s.FD, s.Offset, s.Length, s.Advice)
		return &r
	case *FDAllocateSyscall:
		r := *cast[*FDAllocateSyscall](syscall)
		r.Errno = system.FDAllocate(ctx, s.FD, s.Offset, s.Length)
		return &r
	case *FDCloseSyscall:
		r := *cast[*FDCloseSyscall](syscall)
		r.Errno = system.FDClose(ctx, s.FD)
		return &r
	case *FDDataSyncSyscall:
		r := *cast[*FDDataSyncSyscall](syscall)
		r.Errno = system.FDDataSync(ctx, s.FD)
		return &r
	case *FDStatGetSyscall:
		r := *cast[*FDStatGetSyscall](syscall)
		r.Stat, r.Errno = system.FDStatGet(ctx, s.FD)
		return &r
	case *FDStatSetFlagsSyscall:
		r := *cast[*FDStatSetFlagsSyscall](syscall)
		r.Errno = system.FDStatSetFlags(ctx, s.FD, s.Flags)
		return &r
	case *FDStatSetRightsSyscall:
		r := *cast[*FDStatSetRightsSyscall](syscall)
		r.Errno = system.FDStatSetRights(ctx, s.FD, s.RightsBase, s.RightsInheriting)
		return &r
	case *FDFileStatGetSyscall:
		r := *cast[*FDFileStatGetSyscall](syscall)
		r.Stat, r.Errno = system.FDFileStatGet(ctx, s.FD)
		return &r
	case *FDFileStatSetSizeSyscall:
		r := *cast[*FDFileStatSetSizeSyscall](syscall)
		r.Errno = system.FDFileStatSetSize(ctx, s.FD, s.Size)
		return &r
	case *FDFileStatSetTimesSyscall:
		r := *cast[*FDFileStatSetTimesSyscall](syscall)
		r.Errno = system.FDFileStatSetTimes(ctx, s.FD, s.AccessTime, s.ModifyTime, s.Flags)
		return &r
	case *FDPreadSyscall:
		r := *cast[*FDPreadSyscall](syscall)
		r.Size, r.Errno = system.FDPread(ctx, s.FD, s.IOVecs, s.Offset)
		return &r
	case *FDPreStatGetSyscall:
		r := *cast[*FDPreStatGetSyscall](syscall)
		r.Stat, r.Errno = system.FDPreStatGet(ctx, s.FD)
		return &r
	case *FDPreStatDirNameSyscall:
		r := *cast[*FDPreStatDirNameSyscall](syscall)
		r.Name, r.Errno = system.FDPreStatDirName(ctx, s.FD)
		return &r
	case *FDPwriteSyscall:
		r := *cast[*FDPwriteSyscall](syscall)
		r.Size, r.Errno = system.FDPwrite(ctx, s.FD, s.IOVecs, s.Offset)
		return &r
	case *FDReadSyscall:
		r := *cast[*FDReadSyscall](syscall)
		r.Size, r.Errno = system.FDRead(ctx, s.FD, s.IOVecs)
		return &r
	case *FDReadDirSyscall:
		r := *cast[*FDReadDirSyscall](syscall)
		var n int
		n, r.Errno = system.FDReadDir(ctx, s.FD, s.Entries, s.Cookie, s.BufferSizeBytes)
		if n >= 0 && n < len(s.Entries) {
			s.Entries = s.Entries[:n]
		} else {
			panic("not implemented")
		}
		return &r
	case *FDRenumberSyscall:
		r := *cast[*FDRenumberSyscall](syscall)
		r.Errno = system.FDRenumber(ctx, s.From, s.To)
		return &r
	case *FDSeekSyscall:
		r := *cast[*FDSeekSyscall](syscall)
		r.Size, r.Errno = system.FDSeek(ctx, s.FD, s.Offset, s.Whence)
		return &r
	case *FDSyncSyscall:
		r := *cast[*FDSyncSyscall](syscall)
		r.Errno = system.FDSync(ctx, s.FD)
		return &r
	case *FDTellSyscall:
		r := *cast[*FDTellSyscall](syscall)
		r.Size, r.Errno = system.FDTell(ctx, s.FD)
		return &r
	case *FDWriteSyscall:
		r := *cast[*FDWriteSyscall](syscall)
		r.Size, r.Errno = system.FDWrite(ctx, s.FD, s.IOVecs)
		return &r
	case *PathCreateDirectorySyscall:
		r := *cast[*PathCreateDirectorySyscall](syscall)
		r.Errno = system.PathCreateDirectory(ctx, s.FD, s.Path)
		return &r
	case *PathFileStatGetSyscall:
		r := *cast[*PathFileStatGetSyscall](syscall)
		r.Stat, r.Errno = system.PathFileStatGet(ctx, s.FD, s.LookupFlags, s.Path)
		return &r
	case *PathFileStatSetTimesSyscall:
		r := *cast[*PathFileStatSetTimesSyscall](syscall)
		r.Errno = system.PathFileStatSetTimes(ctx, s.FD, s.LookupFlags, s.Path, s.AccessTime, s.ModifyTime, s.Flags)
		return &r
	case *PathLinkSyscall:
		r := *cast[*PathLinkSyscall](syscall)
		r.Errno = system.PathLink(ctx, s.OldFD, s.OldFlags, s.OldPath, s.NewFD, s.NewPath)
		return &r
	case *PathOpenSyscall:
		r := *cast[*PathOpenSyscall](syscall)
		r.NewFD, r.Errno = system.PathOpen(ctx, s.FD, s.DirFlags, s.Path, s.OpenFlags, s.RightsBase, s.RightsInheriting, s.FDFlags)
		return &r
	case *PathReadLinkSyscall:
		r := *cast[*PathReadLinkSyscall](syscall)
		var n int
		n, r.Errno = system.PathReadLink(ctx, s.FD, s.Path, s.Output)
		if n >= 0 && n <= len(s.Output) {
			s.Output = s.Output[:n]
		} else {
			panic("not implemented")
		}
		return &r
	case *PathRemoveDirectorySyscall:
		r := *cast[*PathRemoveDirectorySyscall](syscall)
		r.Errno = system.PathRemoveDirectory(ctx, s.FD, s.Path)
		return &r
	case *PathRenameSyscall:
		r := *cast[*PathRenameSyscall](syscall)
		r.Errno = system.PathRename(ctx, s.FD, s.OldPath, s.NewFD, s.NewPath)
		return &r
	case *PathSymlinkSyscall:
		r := *cast[*PathSymlinkSyscall](syscall)
		r.Errno = system.PathSymlink(ctx, s.OldPath, s.FD, s.NewPath)
		return &r
	case *PathUnlinkFileSyscall:
		r := *cast[*PathUnlinkFileSyscall](syscall)
		r.Errno = system.PathUnlinkFile(ctx, s.FD, s.Path)
		return &r
	case *PollOneOffSyscall:
		r := *cast[*PollOneOffSyscall](syscall)
		var n int
		n, r.Errno = system.PollOneOff(ctx, s.Subscriptions, s.Events)
		if n >= 0 && n <= len(s.Events) {
			s.Events = s.Events[:n]
		} else {
			panic("not implemented")
		}
		return &r
	case *ProcExitSyscall:
		r := *cast[*ProcExitSyscall](syscall)
		r.Errno = system.ProcExit(ctx, s.ExitCode)
		return &r
	case *ProcRaiseSyscall:
		r := *cast[*ProcRaiseSyscall](syscall)
		r.Errno = system.ProcRaise(ctx, s.Signal)
		return &r
	case *SchedYieldSyscall:
		r := *cast[*SchedYieldSyscall](syscall)
		r.Errno = system.SchedYield(ctx)
		return &r
	case *RandomGetSyscall:
		r := *cast[*RandomGetSyscall](syscall)
		r.Errno = system.RandomGet(ctx, s.B)
		return &r
	case *SockAcceptSyscall:
		r := *cast[*SockAcceptSyscall](syscall)
		r.NewFD, r.Peer, r.Addr, r.Errno = system.SockAccept(ctx, s.FD, s.Flags)
		return &r
	case *SockShutdownSyscall:
		r := *cast[*SockShutdownSyscall](syscall)
		r.Errno = system.SockShutdown(ctx, s.FD, s.Flags)
		return &r
	case *SockRecvSyscall:
		r := *cast[*SockRecvSyscall](syscall)
		r.Size, r.OFlags, s.Errno = system.SockRecv(ctx, s.FD, s.IOVecs, s.IFlags)
		return &r
	case *SockSendSyscall:
		r := *cast[*SockSendSyscall](syscall)
		r.Size, r.Errno = system.SockSend(ctx, s.FD, s.IOVecs, s.IFlags)
		return &r
	case *SockOpenSyscall:
		r := *cast[*SockOpenSyscall](syscall)
		r.FD, r.Errno = system.SockOpen(ctx, s.Family, s.SocketType, s.Protocol, s.RightsBase, s.RightsInheriting)
		return &r
	case *SockBindSyscall:
		r := *cast[*SockBindSyscall](syscall)
		r.Addr, r.Errno = system.SockBind(ctx, s.FD, s.Addr)
		return &r
	case *SockConnectSyscall:
		r := *cast[*SockConnectSyscall](syscall)
		r.Addr, r.Errno = system.SockConnect(ctx, s.FD, s.Addr)
		return &r
	case *SockListenSyscall:
		r := *cast[*SockListenSyscall](syscall)
		r.Errno = system.SockListen(ctx, s.FD, s.Backlog)
		return &r
	case *SockSendToSyscall:
		r := *cast[*SockSendToSyscall](syscall)
		r.Size, r.Errno = system.SockSendTo(ctx, s.FD, s.IOVecs, s.IFlags, s.Addr)
		return &r
	case *SockRecvFromSyscall:
		r := *cast[*SockRecvFromSyscall](syscall)
		r.Size, r.OFlags, r.Addr, r.Errno = system.SockRecvFrom(ctx, s.FD, s.IOVecs, s.IFlags)
		return &r
	case *SockGetOptSyscall:
		r := *cast[*SockGetOptSyscall](syscall)
		r.Value, r.Errno = system.SockGetOpt(ctx, s.FD, s.Level, s.Option)
		return &r
	case *SockSetOptSyscall:
		r := *cast[*SockSetOptSyscall](syscall)
		r.Errno = system.SockSetOpt(ctx, s.FD, s.Level, s.Option, s.Value)
		return &r
	case *SockLocalAddressSyscall:
		r := *cast[*SockLocalAddressSyscall](syscall)
		r.Addr, r.Errno = system.SockLocalAddress(ctx, s.FD)
		return &r
	case *SockRemoteAddressSyscall:
		r := *cast[*SockRemoteAddressSyscall](syscall)
		r.Addr, r.Errno = system.SockRemoteAddress(ctx, s.FD)
		return &r
	case *SockAddressInfoSyscall:
		r := *cast[*SockAddressInfoSyscall](syscall)
		var n int
		n, r.Errno = system.SockAddressInfo(ctx, s.Name, s.Service, s.Hints, s.Res)
		if n >= 0 && n <= len(s.Res) {
			s.Res = s.Res[:n]
		} else {
			panic("not implemented")
		}
		return &r
	default:
		panic("not implemented")
	}
}

// resultsSystem is the inverse to call. You can use it to pull
// Syscall results through a wasi.System.
type resultsSystem struct{ Syscall }

func cast[T Syscall](s Syscall) T {
	t, ok := s.(T)
	if !ok {
		panic("unexpected syscall")
	}
	return t
}

func (p *resultsSystem) ArgsSizesGet(ctx context.Context) (argCount int, stringBytes int, errno wasi.Errno) {
	s := cast[*ArgsSizesGetSyscall](p.Syscall)
	return s.ArgCount, s.StringBytes, s.Errno
}

func (p *resultsSystem) ArgsGet(ctx context.Context) ([]string, wasi.Errno) {
	s := cast[*ArgsGetSyscall](p.Syscall)
	return s.Args, s.Errno
}

func (p *resultsSystem) EnvironSizesGet(ctx context.Context) (argCount int, stringBytes int, errno wasi.Errno) {
	s := cast[*EnvironSizesGetSyscall](p.Syscall)
	return s.EnvCount, s.StringBytes, s.Errno
}

func (p *resultsSystem) EnvironGet(ctx context.Context) ([]string, wasi.Errno) {
	s := cast[*EnvironGetSyscall](p.Syscall)
	return s.Env, s.Errno
}

func (p *resultsSystem) ClockResGet(ctx context.Context, id wasi.ClockID) (wasi.Timestamp, wasi.Errno) {
	s := cast[*ClockResGetSyscall](p.Syscall)
	return s.Timestamp, s.Errno
}

func (p *resultsSystem) ClockTimeGet(ctx context.Context, id wasi.ClockID, precision wasi.Timestamp) (wasi.Timestamp, wasi.Errno) {
	s := cast[*ClockTimeGetSyscall](p.Syscall)
	return s.Timestamp, s.Errno
}

func (p *resultsSystem) FDAdvise(ctx context.Context, fd wasi.FD, offset wasi.FileSize, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	s := cast[*FDAdviseSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDAllocate(ctx context.Context, fd wasi.FD, offset wasi.FileSize, length wasi.FileSize) wasi.Errno {
	s := cast[*FDAllocateSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDClose(ctx context.Context, fd wasi.FD) wasi.Errno {
	s := cast[*FDCloseSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDDataSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	s := cast[*FDDataSyncSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDStatGet(ctx context.Context, fd wasi.FD) (wasi.FDStat, wasi.Errno) {
	s := cast[*FDStatGetSyscall](p.Syscall)
	return s.Stat, s.Errno
}

func (p *resultsSystem) FDStatSetFlags(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) wasi.Errno {
	s := cast[*FDStatSetFlagsSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDStatSetRights(ctx context.Context, fd wasi.FD, rightsBase, rightsInheriting wasi.Rights) wasi.Errno {
	s := cast[*FDStatSetRightsSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDFileStatGet(ctx context.Context, fd wasi.FD) (wasi.FileStat, wasi.Errno) {
	s := cast[*FDFileStatGetSyscall](p.Syscall)
	return s.Stat, s.Errno
}

func (p *resultsSystem) FDFileStatSetSize(ctx context.Context, fd wasi.FD, size wasi.FileSize) wasi.Errno {
	s := cast[*FDFileStatSetSizeSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDFileStatSetTimes(ctx context.Context, fd wasi.FD, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	s := cast[*FDFileStatSetTimesSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDPread(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	s := cast[*FDPreadSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *resultsSystem) FDPreStatGet(ctx context.Context, fd wasi.FD) (wasi.PreStat, wasi.Errno) {
	s := cast[*FDPreStatGetSyscall](p.Syscall)
	return s.Stat, s.Errno
}

func (p *resultsSystem) FDPreStatDirName(ctx context.Context, fd wasi.FD) (string, wasi.Errno) {
	s := cast[*FDPreStatDirNameSyscall](p.Syscall)
	return s.Name, s.Errno
}

func (p *resultsSystem) FDPwrite(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	s := cast[*FDPwriteSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *resultsSystem) FDRead(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	s := cast[*FDReadSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *resultsSystem) FDReadDir(ctx context.Context, fd wasi.FD, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	s := cast[*FDReadDirSyscall](p.Syscall)
	return len(s.Entries), s.Errno
}

func (p *resultsSystem) FDRenumber(ctx context.Context, from, to wasi.FD) wasi.Errno {
	s := cast[*FDRenumberSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDSeek(ctx context.Context, fd wasi.FD, offset wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	s := cast[*FDSeekSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *resultsSystem) FDSync(ctx context.Context, fd wasi.FD) wasi.Errno {
	s := cast[*FDSyncSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) FDTell(ctx context.Context, fd wasi.FD) (wasi.FileSize, wasi.Errno) {
	s := cast[*FDTellSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *resultsSystem) FDWrite(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	s := cast[*FDWriteSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *resultsSystem) PathCreateDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	s := cast[*PathCreateDirectorySyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) PathFileStatGet(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	s := cast[*PathFileStatGetSyscall](p.Syscall)
	return s.Stat, s.Errno
}

func (p *resultsSystem) PathFileStatSetTimes(ctx context.Context, fd wasi.FD, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	s := cast[*PathFileStatSetTimesSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) PathLink(ctx context.Context, oldFD wasi.FD, oldFlags wasi.LookupFlags, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	s := cast[*PathLinkSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) PathOpen(ctx context.Context, fd wasi.FD, dirFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (wasi.FD, wasi.Errno) {
	s := cast[*PathOpenSyscall](p.Syscall)
	return s.NewFD, s.Errno
}

func (p *resultsSystem) PathReadLink(ctx context.Context, fd wasi.FD, path string, buffer []byte) (int, wasi.Errno) {
	s := cast[*PathReadLinkSyscall](p.Syscall)
	return len(s.Output), s.Errno
}

func (p *resultsSystem) PathRemoveDirectory(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	s := cast[*PathRemoveDirectorySyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) PathRename(ctx context.Context, fd wasi.FD, oldPath string, newFD wasi.FD, newPath string) wasi.Errno {
	s := cast[*PathRenameSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) PathSymlink(ctx context.Context, oldPath string, fd wasi.FD, newPath string) wasi.Errno {
	s := cast[*PathSymlinkSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) PathUnlinkFile(ctx context.Context, fd wasi.FD, path string) wasi.Errno {
	s := cast[*PathUnlinkFileSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) PollOneOff(ctx context.Context, subscriptions []wasi.Subscription, events []wasi.Event) (int, wasi.Errno) {
	s := cast[*PollOneOffSyscall](p.Syscall)
	return len(s.Events), s.Errno
}

func (p *resultsSystem) ProcExit(ctx context.Context, exitCode wasi.ExitCode) wasi.Errno {
	s := cast[*ProcExitSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) ProcRaise(ctx context.Context, signal wasi.Signal) wasi.Errno {
	s := cast[*ProcRaiseSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) SchedYield(ctx context.Context) wasi.Errno {
	s := cast[*SchedYieldSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) RandomGet(ctx context.Context, b []byte) wasi.Errno {
	s := cast[*RandomGetSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) SockAccept(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) (wasi.FD, wasi.SocketAddress, wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockAcceptSyscall](p.Syscall)
	return s.NewFD, s.Peer, s.Addr, s.Errno
}

func (p *resultsSystem) SockRecv(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	s := cast[*SockRecvSyscall](p.Syscall)
	return s.Size, s.OFlags, s.Errno
}

func (p *resultsSystem) SockSend(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	s := cast[*SockSendSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *resultsSystem) SockShutdown(ctx context.Context, fd wasi.FD, flags wasi.SDFlags) wasi.Errno {
	s := cast[*SockShutdownSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) Close(ctx context.Context) error {
	return nil
}

func (p *resultsSystem) SockOpen(ctx context.Context, family wasi.ProtocolFamily, socketType wasi.SocketType, protocol wasi.Protocol, rightsBase, rightsInheriting wasi.Rights) (wasi.FD, wasi.Errno) {
	s := cast[*SockOpenSyscall](p.Syscall)
	return s.FD, s.Errno
}

func (p *resultsSystem) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockBindSyscall](p.Syscall)
	return s.Addr, s.Errno
}

func (p *resultsSystem) SockConnect(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockConnectSyscall](p.Syscall)
	return s.Addr, s.Errno
}

func (p *resultsSystem) SockListen(ctx context.Context, fd wasi.FD, backlog int) wasi.Errno {
	s := cast[*SockListenSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) SockSendTo(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	s := cast[*SockSendToSyscall](p.Syscall)
	return s.Size, s.Errno
}

func (p *resultsSystem) SockRecvFrom(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockRecvFromSyscall](p.Syscall)
	return s.Size, s.OFlags, s.Addr, s.Errno
}

func (p *resultsSystem) SockGetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	s := cast[*SockGetOptSyscall](p.Syscall)
	return s.Value, s.Errno
}

func (p *resultsSystem) SockSetOpt(ctx context.Context, fd wasi.FD, level wasi.SocketOptionLevel, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	s := cast[*SockSetOptSyscall](p.Syscall)
	return s.Errno
}

func (p *resultsSystem) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockLocalAddressSyscall](p.Syscall)
	return s.Addr, s.Errno
}

func (p *resultsSystem) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	s := cast[*SockRemoteAddressSyscall](p.Syscall)
	return s.Addr, s.Errno
}

func (p *resultsSystem) SockAddressInfo(ctx context.Context, name, service string, hints wasi.AddressInfo, results []wasi.AddressInfo) (int, wasi.Errno) {
	s := cast[*SockAddressInfoSyscall](p.Syscall)
	return len(s.Res), s.Errno
}

var _ wasi.System = (*resultsSystem)(nil)
var _ wasi.SocketsExtension = (*resultsSystem)(nil)

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
