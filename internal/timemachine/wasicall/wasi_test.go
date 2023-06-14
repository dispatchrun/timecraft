package wasicall

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero/sys"
)

var syscalls = []Syscall{
	&ArgsSizesGetSyscall{},
	&ArgsSizesGetSyscall{ArgCount: 3, StringBytes: 64, Errno: wasi.ESUCCESS},
	&ArgsGetSyscall{Args: []string{"--help"}, Errno: wasi.ESUCCESS},
	&ArgsGetSyscall{Errno: wasi.ENOTSUP},
	&EnvironSizesGetSyscall{},
	&EnvironSizesGetSyscall{EnvCount: 1, StringBytes: 2, Errno: wasi.ESUCCESS},
	&ClockResGetSyscall{ClockID: wasi.Realtime, Timestamp: math.MaxUint64},
	&ClockResGetSyscall{ClockID: wasi.Monotonic, Timestamp: 1111111},
	&ClockResGetSyscall{ClockID: wasi.Monotonic, Errno: wasi.EINVAL},
	&ClockTimeGetSyscall{ClockID: wasi.Realtime, Precision: wasi.Timestamp(time.Microsecond), Timestamp: wasi.Timestamp(time.Now().UnixNano())},
	&ClockTimeGetSyscall{ClockID: wasi.Monotonic, Precision: wasi.Timestamp(time.Nanosecond), Timestamp: 1},
	&ClockTimeGetSyscall{ClockID: wasi.Monotonic, Precision: wasi.Timestamp(time.Nanosecond), Errno: wasi.EINVAL},
	&FDAdviseSyscall{FD: 1, Offset: 4096, Length: 64, Advice: wasi.WillNeed},
	&FDAdviseSyscall{FD: 1, Offset: 4096, Length: 64, Advice: wasi.WillNeed, Errno: wasi.ENOTCAPABLE},
	&FDAdviseSyscall{},
	&FDAllocateSyscall{FD: 11, Offset: 1 << 30, Length: 1},
	&FDAllocateSyscall{},
	&FDAllocateSyscall{Errno: wasi.ENOTSUP},
	&FDCloseSyscall{FD: 23},
	&FDCloseSyscall{},
	&FDCloseSyscall{Errno: wasi.EBADF},
	&FDDataSyncSyscall{FD: math.MaxInt32},
	&FDDataSyncSyscall{},
	&FDDataSyncSyscall{Errno: wasi.EBADF},
	&FDStatGetSyscall{FD: 1, Stat: wasi.FDStat{FileType: wasi.SocketStreamType, Flags: wasi.NonBlock, RightsBase: wasi.SockListenRights, RightsInheriting: wasi.SockConnectionRights | wasi.SockListenRights}},
	&FDStatGetSyscall{FD: 23, Stat: wasi.FDStat{}, Errno: wasi.EBADF},
	&FDStatGetSyscall{FD: 1, Stat: wasi.FDStat{FileType: 2, Flags: 3, RightsBase: 4, RightsInheriting: 5}, Errno: 6},
	&FDStatSetFlagsSyscall{FD: math.MaxInt32 - 1, Flags: wasi.Append | wasi.Sync},
	&FDStatSetFlagsSyscall{},
	&FDStatSetFlagsSyscall{FD: 4, Errno: wasi.ENOENT},
	&FDStatSetRightsSyscall{FD: 1, RightsBase: 2, RightsInheriting: 3, Errno: 4},
	&FDStatSetRightsSyscall{FD: 5, RightsBase: wasi.AllRights, RightsInheriting: wasi.FileRights},
	&FDStatSetRightsSyscall{FD: 5, Errno: wasi.ENOTCAPABLE},
	&FDFileStatGetSyscall{FD: 6, Stat: wasi.FileStat{Device: ^wasi.Device(0), INode: ^wasi.INode(0), FileType: ^wasi.FileType(0), NLink: ^wasi.LinkCount(0), Size: ^wasi.FileSize(0), AccessTime: ^wasi.Timestamp(0), ModifyTime: ^wasi.Timestamp(0)}},
	&FDFileStatGetSyscall{FD: 1, Stat: wasi.FileStat{Device: 2, INode: 3, FileType: 4, NLink: 5, Size: 6, AccessTime: 7, ModifyTime: 8}},
	&FDFileStatGetSyscall{},
	&FDFileStatSetSizeSyscall{FD: 1, Size: 4096, Errno: wasi.ESUCCESS},
	&FDFileStatSetSizeSyscall{FD: 1, Size: 2, Errno: 3},
	&FDFileStatSetSizeSyscall{},
	&FDFileStatSetTimesSyscall{FD: 1, AccessTime: 2, ModifyTime: 3, Flags: wasi.AccessTimeNow | wasi.ModifyTimeNow},
	&FDFileStatSetTimesSyscall{},
	&FDPreadSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foo"), []byte("bar")}, Size: 6, Errno: wasi.ESUCCESS},
	&FDPreadSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foobar")}, Offset: 3, Size: 6, Errno: wasi.ESUCCESS},
	&FDPreadSyscall{},
	&FDPreStatGetSyscall{FD: 9, Stat: wasi.PreStat{Type: ^wasi.PreOpenType(0), PreStatDir: wasi.PreStatDir{NameLength: ^wasi.Size(0)}}, Errno: wasi.ESUCCESS},
	&FDPreStatGetSyscall{FD: 1, Stat: wasi.PreStat{Type: 2, PreStatDir: wasi.PreStatDir{NameLength: 3}}, Errno: 4},
	&FDPreStatGetSyscall{},
	&FDPwriteSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foo"), []byte("bar")}, Size: 6, Errno: wasi.ESUCCESS},
	&FDPwriteSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foobar")}, Offset: 3, Size: 6, Errno: wasi.ESUCCESS},
	&FDPwriteSyscall{},
	&FDReadSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foo"), []byte("bar")}, Size: 6, Errno: wasi.ESUCCESS},
	&FDReadSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foobar")}, Size: 6, Errno: wasi.ESUCCESS},
	&FDReadSyscall{},
	&FDReadDirSyscall{FD: 1, Entries: []wasi.DirEntry{{Next: ^wasi.DirCookie(0), INode: ^wasi.INode(0), Type: ^wasi.FileType(0), Name: []byte("/foobar")}}, Cookie: ^wasi.DirCookie(0), BufferSizeBytes: math.MaxInt, Errno: wasi.ESUCCESS},
	&FDReadDirSyscall{FD: 1, Entries: []wasi.DirEntry{{Next: 2, INode: 3, Type: 4, Name: []byte("5")}, {Next: 6, INode: 7, Type: 8, Name: []byte("9")}}, Cookie: 10, BufferSizeBytes: 11, Errno: 12},
	&FDReadDirSyscall{},
	&FDRenumberSyscall{From: ^wasi.FD(0), To: ^wasi.FD(0), Errno: ^wasi.Errno(0)},
	&FDRenumberSyscall{From: 1, To: 2, Errno: 3},
	&FDRenumberSyscall{},
	&FDSeekSyscall{FD: math.MaxInt32, Offset: ^wasi.FileDelta(0), Whence: ^wasi.Whence(0), Errno: 45},
	&FDSeekSyscall{FD: 1, Offset: 2, Whence: 3, Errno: 4, Size: 5},
	&FDSeekSyscall{FD: math.MinInt32, Offset: -1, Whence: 0, Errno: 4, Size: ^wasi.FileSize(0)},
	&FDSeekSyscall{},
	&FDSyncSyscall{FD: math.MaxInt32},
	&FDSyncSyscall{FD: 1, Errno: 2},
	&FDSyncSyscall{},
	&FDTellSyscall{FD: math.MaxInt32, Size: math.MaxUint64},
	&FDTellSyscall{FD: 1, Size: 2, Errno: 3},
	&FDTellSyscall{},
	&FDWriteSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foo"), []byte("bar")}, Size: 6, Errno: wasi.ESUCCESS},
	&FDWriteSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foobar")}, Size: 6, Errno: wasi.ESUCCESS},
	&FDWriteSyscall{},
	&PathCreateDirectorySyscall{Path: "foobar", FD: 1, Errno: 2},
	&PathCreateDirectorySyscall{},
	&PathFileStatGetSyscall{FD: 23, LookupFlags: wasi.SymlinkFollow, Path: "foobar", Stat: wasi.FileStat{Device: 1, INode: 2, FileType: 3, NLink: 4, Size: 5, AccessTime: 6, ModifyTime: 7, ChangeTime: 8}, Errno: 9},
	&PathFileStatGetSyscall{},
	&PathFileStatSetTimesSyscall{FD: 1, LookupFlags: ^wasi.LookupFlags(0), Path: "foo", AccessTime: 2, ModifyTime: 3, Flags: wasi.AccessTime, Errno: 4},
	&PathFileStatSetTimesSyscall{},
	&PathLinkSyscall{OldFD: 1, OldFlags: wasi.SymlinkFollow, OldPath: "foo", NewFD: 3, NewPath: "bar", Errno: 4},
	&PathLinkSyscall{},
	&PathOpenSyscall{FD: 1, DirFlags: 2, Path: "foo", OpenFlags: wasi.OpenCreate, RightsBase: wasi.FileRights, RightsInheriting: wasi.FileRights &^ wasi.FDSeekRight, FDFlags: wasi.NonBlock, NewFD: 11111, Errno: 2222},
	&PathOpenSyscall{},
	&PathReadLinkSyscall{FD: 1, Path: "foo", Output: []byte("bar"), Errno: 2},
	&PathReadLinkSyscall{Output: []byte{}},
	&PathRemoveDirectorySyscall{FD: 1, Path: "foo", Errno: 2},
	&PathRemoveDirectorySyscall{},
	&PathRenameSyscall{FD: 1, OldPath: "foo", NewFD: 2, NewPath: "bar", Errno: 3},
	&PathRenameSyscall{},
	&PathSymlinkSyscall{OldPath: "foo", FD: 1, NewPath: "bar", Errno: 2},
	&PathSymlinkSyscall{},
	&PathUnlinkFileSyscall{FD: ^wasi.FD(0), Path: "foobar", Errno: ^wasi.Errno(0)},
	&PathUnlinkFileSyscall{},
	&PollOneOffSyscall{
		Subscriptions: []wasi.Subscription{
			wasi.MakeSubscriptionFDReadWrite(^wasi.UserData(0), wasi.FDReadEvent, wasi.SubscriptionFDReadWrite{FD: 2}),
			wasi.MakeSubscriptionFDReadWrite(^wasi.UserData(0)-1, wasi.FDWriteEvent, wasi.SubscriptionFDReadWrite{FD: 3}),
			wasi.MakeSubscriptionClock(^wasi.UserData(0)-2, wasi.SubscriptionClock{ID: wasi.Monotonic, Timeout: ^wasi.Timestamp(0), Precision: 1, Flags: wasi.Abstime}),
			wasi.MakeSubscriptionClock(^wasi.UserData(0)-3, wasi.SubscriptionClock{ID: ^wasi.ClockID(0), Flags: ^wasi.SubscriptionClockFlags(0)}),
		},
		Events: []wasi.Event{
			{UserData: ^wasi.UserData(0), Errno: wasi.ECANCELED, EventType: wasi.FDReadEvent, FDReadWrite: wasi.EventFDReadWrite{NBytes: 1, Flags: wasi.Hangup}},
			{UserData: ^wasi.UserData(0) - 3, EventType: wasi.ClockEvent},
		},
		Errno: wasi.ESUCCESS,
	},
	&PollOneOffSyscall{},
	&ProcExitSyscall{ExitCode: ^wasi.ExitCode(0), Errno: wasi.ESUCCESS},
	&ProcExitSyscall{},
	&ProcRaiseSyscall{Signal: ^wasi.Signal(0), Errno: 2},
	&ProcRaiseSyscall{},
	&SchedYieldSyscall{Errno: 1},
	&SchedYieldSyscall{},
	&RandomGetSyscall{B: []byte("xyzzy"), Errno: 1},
	&RandomGetSyscall{B: []byte{}},
	&SockAcceptSyscall{FD: 1, Flags: wasi.NonBlock, NewFD: 2, Peer: &wasi.Inet4Address{Addr: [4]byte{3, 4, 5, 6}, Port: 7}, Addr: &wasi.Inet6Address{Addr: [16]byte{8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}, Port: 20}, Errno: 21},
	&SockAcceptSyscall{},
	&SockShutdownSyscall{FD: 1, Flags: 2, Errno: 3},
	&SockShutdownSyscall{Flags: ^wasi.SDFlags(0)},
	&SockShutdownSyscall{},
	&SockRecvSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foo"), []byte("bar")}, IFlags: wasi.RecvPeek, Size: 6, OFlags: wasi.RecvDataTruncated, Errno: wasi.ENOTSOCK},
	&SockRecvSyscall{},
	&SockSendSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foo"), []byte("bar")}, IFlags: 2, Size: 6, Errno: wasi.ENOTSOCK},
	&SockSendSyscall{},
	&SockOpenSyscall{Family: ^wasi.ProtocolFamily(0), SocketType: ^wasi.SocketType(0), Protocol: ^wasi.Protocol(0), RightsBase: ^wasi.Rights(0), RightsInheriting: 1, FD: 2, Errno: 3},
	&SockOpenSyscall{Family: wasi.InetFamily, SocketType: wasi.StreamSocket, Protocol: wasi.TCPProtocol, RightsBase: wasi.SockListenRights, RightsInheriting: wasi.SockConnectionRights, FD: 1, Errno: wasi.ESUCCESS},
	&SockOpenSyscall{},
	&SockBindSyscall{FD: 1, Bind: &wasi.UnixAddress{Name: "foo"}, Addr: &wasi.UnixAddress{"bar"}, Errno: 2},
	&SockBindSyscall{FD: 1, Bind: &wasi.Inet4Address{Addr: [4]byte{127, 0, 0, 1}, Port: 0}, Addr: &wasi.Inet4Address{Addr: [4]byte{127, 0, 0, 1}, Port: 45980}, Errno: wasi.ESUCCESS},
	&SockBindSyscall{},
	&SockConnectSyscall{FD: 1, Peer: &wasi.Inet4Address{Addr: [4]byte{127, 0, 0, 1}, Port: 0}, Addr: &wasi.Inet4Address{Addr: [4]byte{127, 0, 0, 1}, Port: 45980}, Errno: wasi.ESUCCESS},
	&SockConnectSyscall{},
	&SockListenSyscall{FD: 1, Backlog: 2, Errno: 3},
	&SockListenSyscall{},
	&SockSendToSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foo"), []byte("bar")}, IFlags: ^wasi.SIFlags(0), Addr: &wasi.Inet4Address{Addr: [4]byte{1, 2, 3, 4}, Port: 5}, Size: 6, Errno: 7},
	&SockSendToSyscall{},
	&SockRecvFromSyscall{FD: 1, IOVecs: []wasi.IOVec{[]byte("foo"), []byte("bar")}, IFlags: ^wasi.RIFlags(0), Addr: &wasi.Inet4Address{Addr: [4]byte{1, 2, 3, 4}, Port: 5}, OFlags: 8, Size: 6, Errno: 7},
	&SockRecvFromSyscall{},
	&SockGetOptSyscall{FD: 1, Level: wasi.SocketLevel, Option: wasi.Broadcast, Value: wasi.IntValue(1), Errno: 0},
	&SockGetOptSyscall{FD: 1, Level: ^wasi.SocketOptionLevel(0), Option: ^wasi.SocketOption(0), Value: nil, Errno: 1},
	&SockGetOptSyscall{},
	&SockSetOptSyscall{FD: 1, Level: wasi.SocketLevel, Option: wasi.Broadcast, Value: wasi.IntValue(1), Errno: 0},
	&SockSetOptSyscall{FD: 1, Level: ^wasi.SocketOptionLevel(0), Option: ^wasi.SocketOption(0), Value: nil, Errno: 1},
	&SockSetOptSyscall{},
	&SockLocalAddressSyscall{FD: 1, Addr: &wasi.Inet4Address{Addr: [4]byte{2, 3, 4, 5}, Port: 6}, Errno: 7},
	&SockLocalAddressSyscall{},
	&SockRemoteAddressSyscall{FD: 1, Addr: &wasi.Inet4Address{Addr: [4]byte{2, 3, 4, 5}, Port: 6}, Errno: 7},
	&SockRemoteAddressSyscall{},
	&SockAddressInfoSyscall{
		Name:    "example.com",
		Service: "http",
		Hints: wasi.AddressInfo{
			Flags:      ^wasi.AddressInfoFlags(0),
			Family:     ^wasi.ProtocolFamily(0),
			SocketType: ^wasi.SocketType(0),
			Protocol:   ^wasi.Protocol(0),
		},
		Res: []wasi.AddressInfo{
			{
				Address:       &wasi.Inet4Address{Addr: [4]byte{2, 3, 4, 5}, Port: 6},
				CanonicalName: "foo",
			},
			{
				Address:       &wasi.Inet6Address{Addr: [16]byte{8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23}, Port: 20},
				CanonicalName: "bar",
			},
		},
	},
	&SockAddressInfoSyscall{},
}

func syscallString(s Syscall) string {
	return fmt.Sprintf("%s(%v) => %v", s.ID(), s.Params(), s.Results())
}

func suppressProcExit(s Syscall) {
	pe, ok := s.(*ProcExitSyscall)
	if !ok {
		return
	}
	if err := recover(); err != nil {
		if e, ok := err.(*sys.ExitError); !ok || wasi.ExitCode(e.ExitCode()) != pe.ExitCode {
			panic(err)
		}
	}
}

// call pushes Syscall params into a wasi.System and returns
// a copy of the Syscall instance that includes the results.
func call(ctx context.Context, system wasi.System, syscall Syscall) Syscall {
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
		if n >= 0 && n <= len(s.Entries) {
			r.Entries = s.Entries[:n]
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
			r.Output = s.Output[:n]
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
			r.Events = s.Events[:n]
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
		r.Size, r.OFlags, r.Errno = system.SockRecv(ctx, s.FD, s.IOVecs, s.IFlags)
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
		r.Addr, r.Errno = system.SockBind(ctx, s.FD, s.Bind)
		return &r
	case *SockConnectSyscall:
		r := *cast[*SockConnectSyscall](syscall)
		r.Addr, r.Errno = system.SockConnect(ctx, s.FD, s.Peer)
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
			r.Res = s.Res[:n]
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

func assertSyscallEqual(t *testing.T, actual, expect Syscall) {
	if actual.ID() != expect.ID() {
		t.Fatalf("syscalls did not match: got %s, expect %s", actual.ID(), expect.ID())
	}
	assertSyscallParamsOrResultsEqual(t, "params", actual.Params(), expect.Params())
	assertSyscallParamsOrResultsEqual(t, "results", actual.Results(), expect.Results())
}

func assertSyscallParamsOrResultsEqual(t *testing.T, which string, actual, expect []any) {
	if len(actual) != len(expect) {
		t.Errorf("syscall %s did not match", which)
		t.Log("actual:")
		spew.Dump(actual)
		t.Log("expect:")
		spew.Dump(expect)
	}
	for i := range actual {
		assertSyscallValueEqual(t, fmt.Sprintf("%s[%d]", which, i), actual[i], expect[i])
	}
}

func assertSyscallValueEqual(t *testing.T, which string, actual, expect any) {
	if reflect.DeepEqual(actual, expect) {
		return
	}
	switch a := actual.(type) {
	case wasi.SocketAddress:
		b, ok := expect.(wasi.SocketAddress)
		if !ok {
			t.Fatalf("syscall %s did not match, got %#v, expect %#v", which, actual, expect)
		}
		assertSocketAddressEqual(t, a, b)
	default:
		panic(fmt.Sprintf("not implemented: %T / %T", actual, expect))
	}
}

func assertSocketAddressEqual(t *testing.T, actual, expect wasi.SocketAddress) {
	if actual == nil {
		if expect != nil {
			t.Fatalf("unexpected socket address: got %#v, expect %#v", actual, expect)
		}
		return
	}
	var equal bool
	switch at := actual.(type) {
	case *wasi.Inet4Address:
		et, ok := expect.(*wasi.Inet4Address)
		equal = ok && *at == *et
	case *wasi.Inet6Address:
		et, ok := expect.(*wasi.Inet6Address)
		equal = ok && *at == *et
	case *wasi.UnixAddress:
		et, ok := expect.(*wasi.UnixAddress)
		equal = ok && *at == *et
	}
	if !equal {
		t.Fatalf("unexpected socket address: got %#v, expect %#v", actual, expect)
	}
}
