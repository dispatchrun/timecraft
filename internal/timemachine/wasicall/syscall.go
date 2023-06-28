package wasicall

import (
	"fmt"

	. "github.com/stealthrocket/wasi-go"
)

// Syscall is a system call.
//
// It carries an identifier for the syscall, the inputs passed
// to the underlying system call, and the value(s) it returns.
type Syscall interface {
	// ID is the syscall identifier.
	ID() SyscallID

	// Params is the set of input parameters.
	Params() []any

	// Results is the set of return values.
	Results() []any

	// Error is the system call error, or ESUCCESS if no
	// error occurred.
	Error() Errno

	private()
}

type ArgsSizesGetSyscall struct {
	ArgCount    int
	StringBytes int
	Errno       Errno
}

func (s *ArgsSizesGetSyscall) ID() SyscallID  { return ArgsSizesGet }
func (s *ArgsSizesGetSyscall) Params() []any  { return nil }
func (s *ArgsSizesGetSyscall) Results() []any { return []any{s.ArgCount, s.StringBytes, s.Errno} }
func (s *ArgsSizesGetSyscall) Error() Errno   { return s.Errno }
func (s *ArgsSizesGetSyscall) private()       {}

type ArgsGetSyscall struct {
	Args  []string
	Errno Errno
}

func (s *ArgsGetSyscall) ID() SyscallID  { return ArgsGet }
func (s *ArgsGetSyscall) Params() []any  { return nil }
func (s *ArgsGetSyscall) Results() []any { return []any{s.Args, s.Errno} }
func (s *ArgsGetSyscall) Error() Errno   { return s.Errno }
func (s *ArgsGetSyscall) private()       {}

type EnvironSizesGetSyscall struct {
	EnvCount    int
	StringBytes int
	Errno       Errno
}

func (s *EnvironSizesGetSyscall) ID() SyscallID  { return EnvironSizesGet }
func (s *EnvironSizesGetSyscall) Params() []any  { return nil }
func (s *EnvironSizesGetSyscall) Results() []any { return []any{s.EnvCount, s.StringBytes, s.Errno} }
func (s *EnvironSizesGetSyscall) Error() Errno   { return s.Errno }
func (s *EnvironSizesGetSyscall) private()       {}

type EnvironGetSyscall struct {
	Env   []string
	Errno Errno
}

func (s *EnvironGetSyscall) ID() SyscallID  { return EnvironGet }
func (s *EnvironGetSyscall) Params() []any  { return nil }
func (s *EnvironGetSyscall) Results() []any { return []any{s.Env, s.Errno} }
func (s *EnvironGetSyscall) Error() Errno   { return s.Errno }
func (s *EnvironGetSyscall) private()       {}

type ClockResGetSyscall struct {
	ClockID   ClockID
	Timestamp Timestamp
	Errno     Errno
}

func (s *ClockResGetSyscall) ID() SyscallID  { return ClockResGet }
func (s *ClockResGetSyscall) Params() []any  { return []any{s.ClockID} }
func (s *ClockResGetSyscall) Results() []any { return []any{s.Timestamp, s.Errno} }
func (s *ClockResGetSyscall) Error() Errno   { return s.Errno }
func (s *ClockResGetSyscall) private()       {}

type ClockTimeGetSyscall struct {
	ClockID   ClockID
	Precision Timestamp
	Timestamp Timestamp
	Errno     Errno
}

func (s *ClockTimeGetSyscall) ID() SyscallID  { return ClockTimeGet }
func (s *ClockTimeGetSyscall) Params() []any  { return []any{s.ClockID, s.Precision} }
func (s *ClockTimeGetSyscall) Results() []any { return []any{s.Timestamp, s.Errno} }
func (s *ClockTimeGetSyscall) Error() Errno   { return s.Errno }
func (s *ClockTimeGetSyscall) private()       {}

type FDAdviseSyscall struct {
	FD     FD
	Offset FileSize
	Length FileSize
	Advice Advice
	Errno  Errno
}

func (s *FDAdviseSyscall) ID() SyscallID  { return FDAdvise }
func (s *FDAdviseSyscall) Params() []any  { return []any{s.FD, s.Offset, s.Length, s.Advice} }
func (s *FDAdviseSyscall) Results() []any { return []any{s.Errno} }
func (s *FDAdviseSyscall) Error() Errno   { return s.Errno }
func (s *FDAdviseSyscall) private()       {}

type FDAllocateSyscall struct {
	FD     FD
	Offset FileSize
	Length FileSize
	Errno  Errno
}

func (s *FDAllocateSyscall) ID() SyscallID  { return FDAllocate }
func (s *FDAllocateSyscall) Params() []any  { return []any{s.FD, s.Offset, s.Length} }
func (s *FDAllocateSyscall) Results() []any { return []any{s.Errno} }
func (s *FDAllocateSyscall) Error() Errno   { return s.Errno }
func (s *FDAllocateSyscall) private()       {}

type FDCloseSyscall struct {
	FD    FD
	Errno Errno
}

func (s *FDCloseSyscall) ID() SyscallID  { return FDClose }
func (s *FDCloseSyscall) Params() []any  { return []any{s.FD} }
func (s *FDCloseSyscall) Results() []any { return []any{s.Errno} }
func (s *FDCloseSyscall) Error() Errno   { return s.Errno }
func (s *FDCloseSyscall) private()       {}

type FDDataSyncSyscall struct {
	FD    FD
	Errno Errno
}

func (s *FDDataSyncSyscall) ID() SyscallID  { return FDDataSync }
func (s *FDDataSyncSyscall) Params() []any  { return []any{s.FD} }
func (s *FDDataSyncSyscall) Results() []any { return []any{s.Errno} }
func (s *FDDataSyncSyscall) Error() Errno   { return s.Errno }
func (s *FDDataSyncSyscall) private()       {}

type FDStatGetSyscall struct {
	FD    FD
	Stat  FDStat
	Errno Errno
}

func (s *FDStatGetSyscall) ID() SyscallID  { return FDStatGet }
func (s *FDStatGetSyscall) Params() []any  { return []any{s.FD} }
func (s *FDStatGetSyscall) Results() []any { return []any{s.Stat, s.Errno} }
func (s *FDStatGetSyscall) Error() Errno   { return s.Errno }
func (s *FDStatGetSyscall) private()       {}

type FDStatSetFlagsSyscall struct {
	FD    FD
	Flags FDFlags
	Errno Errno
}

func (s *FDStatSetFlagsSyscall) ID() SyscallID  { return FDStatSetFlags }
func (s *FDStatSetFlagsSyscall) Params() []any  { return []any{s.FD, s.Flags} }
func (s *FDStatSetFlagsSyscall) Results() []any { return []any{s.Errno} }
func (s *FDStatSetFlagsSyscall) Error() Errno   { return s.Errno }
func (s *FDStatSetFlagsSyscall) private()       {}

type FDStatSetRightsSyscall struct {
	FD               FD
	RightsBase       Rights
	RightsInheriting Rights
	Errno            Errno
}

func (s *FDStatSetRightsSyscall) ID() SyscallID  { return FDStatSetRights }
func (s *FDStatSetRightsSyscall) Params() []any  { return []any{s.FD, s.RightsBase, s.RightsInheriting} }
func (s *FDStatSetRightsSyscall) Results() []any { return []any{s.Errno} }
func (s *FDStatSetRightsSyscall) Error() Errno   { return s.Errno }
func (s *FDStatSetRightsSyscall) private()       {}

type FDFileStatGetSyscall struct {
	FD    FD
	Stat  FileStat
	Errno Errno
}

func (s *FDFileStatGetSyscall) ID() SyscallID  { return FDFileStatGet }
func (s *FDFileStatGetSyscall) Params() []any  { return []any{s.FD} }
func (s *FDFileStatGetSyscall) Results() []any { return []any{s.Stat, s.Errno} }
func (s *FDFileStatGetSyscall) Error() Errno   { return s.Errno }
func (s *FDFileStatGetSyscall) private()       {}

type FDFileStatSetSizeSyscall struct {
	FD    FD
	Size  FileSize
	Errno Errno
}

func (s *FDFileStatSetSizeSyscall) ID() SyscallID  { return FDFileStatSetSize }
func (s *FDFileStatSetSizeSyscall) Params() []any  { return []any{s.FD, s.Size} }
func (s *FDFileStatSetSizeSyscall) Results() []any { return []any{s.Errno} }
func (s *FDFileStatSetSizeSyscall) Error() Errno   { return s.Errno }
func (s *FDFileStatSetSizeSyscall) private()       {}

type FDFileStatSetTimesSyscall struct {
	FD         FD
	AccessTime Timestamp
	ModifyTime Timestamp
	Flags      FSTFlags
	Errno      Errno
}

func (s *FDFileStatSetTimesSyscall) ID() SyscallID { return FDFileStatSetTimes }
func (s *FDFileStatSetTimesSyscall) Params() []any {
	return []any{s.FD, s.AccessTime, s.ModifyTime, s.Flags}
}
func (s *FDFileStatSetTimesSyscall) Results() []any { return []any{s.Errno} }
func (s *FDFileStatSetTimesSyscall) Error() Errno   { return s.Errno }
func (s *FDFileStatSetTimesSyscall) private()       {}

type FDPreadSyscall struct {
	FD     FD
	IOVecs []IOVec
	Offset FileSize
	Size   Size
	Errno  Errno
}

func (s *FDPreadSyscall) ID() SyscallID  { return FDPread }
func (s *FDPreadSyscall) Params() []any  { return []any{s.FD, s.IOVecs, s.Offset} }
func (s *FDPreadSyscall) Results() []any { return []any{s.Size, s.Errno} }
func (s *FDPreadSyscall) Error() Errno   { return s.Errno }
func (s *FDPreadSyscall) private()       {}

type FDPreStatGetSyscall struct {
	FD    FD
	Stat  PreStat
	Errno Errno
}

func (s *FDPreStatGetSyscall) ID() SyscallID  { return FDPreStatGet }
func (s *FDPreStatGetSyscall) Params() []any  { return []any{s.FD} }
func (s *FDPreStatGetSyscall) Results() []any { return []any{s.Stat, s.Errno} }
func (s *FDPreStatGetSyscall) Error() Errno   { return s.Errno }
func (s *FDPreStatGetSyscall) private()       {}

type FDPreStatDirNameSyscall struct {
	FD    FD
	Name  string
	Errno Errno
}

func (s *FDPreStatDirNameSyscall) ID() SyscallID  { return FDPreStatDirName }
func (s *FDPreStatDirNameSyscall) Params() []any  { return []any{s.FD} }
func (s *FDPreStatDirNameSyscall) Results() []any { return []any{s.Name, s.Errno} }
func (s *FDPreStatDirNameSyscall) Error() Errno   { return s.Errno }
func (s *FDPreStatDirNameSyscall) private()       {}

type FDPwriteSyscall struct {
	FD     FD
	IOVecs []IOVec
	Offset FileSize
	Size   Size
	Errno  Errno
}

func (s *FDPwriteSyscall) ID() SyscallID  { return FDPwrite }
func (s *FDPwriteSyscall) Params() []any  { return []any{s.FD, s.IOVecs, s.Offset} }
func (s *FDPwriteSyscall) Results() []any { return []any{s.Size, s.Errno} }
func (s *FDPwriteSyscall) Error() Errno   { return s.Errno }
func (s *FDPwriteSyscall) private()       {}

type FDReadSyscall struct {
	FD     FD
	IOVecs []IOVec
	Size   Size
	Errno  Errno
}

func (s *FDReadSyscall) ID() SyscallID  { return FDRead }
func (s *FDReadSyscall) Params() []any  { return []any{s.FD, s.IOVecs} }
func (s *FDReadSyscall) Results() []any { return []any{s.Size, s.Errno} }
func (s *FDReadSyscall) Error() Errno   { return s.Errno }
func (s *FDReadSyscall) private()       {}

type FDReadDirSyscall struct {
	FD              FD
	Entries         []DirEntry
	Cookie          DirCookie
	BufferSizeBytes int
	Errno           Errno
}

func (s *FDReadDirSyscall) ID() SyscallID  { return FDReadDir }
func (s *FDReadDirSyscall) Params() []any  { return []any{s.FD, s.Entries, s.Cookie, s.BufferSizeBytes} }
func (s *FDReadDirSyscall) Results() []any { return []any{len(s.Entries), s.Errno} }
func (s *FDReadDirSyscall) Error() Errno   { return s.Errno }
func (s *FDReadDirSyscall) private()       {}

type FDRenumberSyscall struct {
	From  FD
	To    FD
	Errno Errno
}

func (s *FDRenumberSyscall) ID() SyscallID  { return FDRenumber }
func (s *FDRenumberSyscall) Params() []any  { return []any{s.From, s.To} }
func (s *FDRenumberSyscall) Results() []any { return []any{s.Errno} }
func (s *FDRenumberSyscall) Error() Errno   { return s.Errno }
func (s *FDRenumberSyscall) private()       {}

type FDSeekSyscall struct {
	FD     FD
	Offset FileDelta
	Whence Whence
	Size   FileSize
	Errno  Errno
}

func (s *FDSeekSyscall) ID() SyscallID  { return FDSeek }
func (s *FDSeekSyscall) Params() []any  { return []any{s.FD, s.Offset, s.Whence} }
func (s *FDSeekSyscall) Results() []any { return []any{s.Size, s.Errno} }
func (s *FDSeekSyscall) Error() Errno   { return s.Errno }
func (s *FDSeekSyscall) private()       {}

type FDSyncSyscall struct {
	FD    FD
	Errno Errno
}

func (s *FDSyncSyscall) ID() SyscallID  { return FDSync }
func (s *FDSyncSyscall) Params() []any  { return []any{s.FD} }
func (s *FDSyncSyscall) Results() []any { return []any{s.Errno} }
func (s *FDSyncSyscall) Error() Errno   { return s.Errno }
func (s *FDSyncSyscall) private()       {}

type FDTellSyscall struct {
	FD    FD
	Size  FileSize
	Errno Errno
}

func (s *FDTellSyscall) ID() SyscallID  { return FDTell }
func (s *FDTellSyscall) Params() []any  { return []any{s.FD} }
func (s *FDTellSyscall) Results() []any { return []any{s.Size, s.Errno} }
func (s *FDTellSyscall) Error() Errno   { return s.Errno }
func (s *FDTellSyscall) private()       {}

type FDWriteSyscall struct {
	FD     FD
	IOVecs []IOVec
	Size   Size
	Errno  Errno
}

func (s *FDWriteSyscall) ID() SyscallID  { return FDWrite }
func (s *FDWriteSyscall) Params() []any  { return []any{s.FD, s.IOVecs} }
func (s *FDWriteSyscall) Results() []any { return []any{s.Size, s.Errno} }
func (s *FDWriteSyscall) Error() Errno   { return s.Errno }
func (s *FDWriteSyscall) private()       {}

type PathCreateDirectorySyscall struct {
	FD    FD
	Path  string
	Errno Errno
}

func (s *PathCreateDirectorySyscall) ID() SyscallID  { return PathCreateDirectory }
func (s *PathCreateDirectorySyscall) Params() []any  { return []any{s.FD, s.Path} }
func (s *PathCreateDirectorySyscall) Results() []any { return []any{s.Errno} }
func (s *PathCreateDirectorySyscall) Error() Errno   { return s.Errno }
func (s *PathCreateDirectorySyscall) private()       {}

type PathFileStatGetSyscall struct {
	FD          FD
	LookupFlags LookupFlags
	Path        string
	Stat        FileStat
	Errno       Errno
}

func (s *PathFileStatGetSyscall) ID() SyscallID  { return PathFileStatGet }
func (s *PathFileStatGetSyscall) Params() []any  { return []any{s.FD, s.LookupFlags, s.Path} }
func (s *PathFileStatGetSyscall) Results() []any { return []any{s.Stat, s.Errno} }
func (s *PathFileStatGetSyscall) Error() Errno   { return s.Errno }
func (s *PathFileStatGetSyscall) private()       {}

type PathFileStatSetTimesSyscall struct {
	FD          FD
	LookupFlags LookupFlags
	Path        string
	AccessTime  Timestamp
	ModifyTime  Timestamp
	Flags       FSTFlags
	Errno       Errno
}

func (s *PathFileStatSetTimesSyscall) ID() SyscallID { return PathFileStatSetTimes }
func (s *PathFileStatSetTimesSyscall) Params() []any {
	return []any{s.FD, s.LookupFlags, s.Path, s.AccessTime, s.ModifyTime, s.Flags}
}
func (s *PathFileStatSetTimesSyscall) Results() []any { return []any{s.Errno} }
func (s *PathFileStatSetTimesSyscall) Error() Errno   { return s.Errno }
func (s *PathFileStatSetTimesSyscall) private()       {}

type PathLinkSyscall struct {
	OldFD    FD
	OldFlags LookupFlags
	OldPath  string
	NewFD    FD
	NewPath  string
	Errno    Errno
}

func (s *PathLinkSyscall) ID() SyscallID { return PathLink }
func (s *PathLinkSyscall) Params() []any {
	return []any{s.OldFD, s.OldFlags, s.OldPath, s.NewFD, s.NewPath}
}
func (s *PathLinkSyscall) Results() []any { return []any{s.Errno} }
func (s *PathLinkSyscall) Error() Errno   { return s.Errno }
func (s *PathLinkSyscall) private()       {}

type PathOpenSyscall struct {
	FD               FD
	DirFlags         LookupFlags
	Path             string
	OpenFlags        OpenFlags
	RightsBase       Rights
	RightsInheriting Rights
	FDFlags          FDFlags
	NewFD            FD
	Errno            Errno
}

func (s *PathOpenSyscall) ID() SyscallID { return PathOpen }
func (s *PathOpenSyscall) Params() []any {
	return []any{s.FD, s.DirFlags, s.Path, s.OpenFlags, s.RightsBase, s.RightsInheriting, s.FDFlags}
}
func (s *PathOpenSyscall) Results() []any { return []any{s.NewFD, s.Errno} }
func (s *PathOpenSyscall) Error() Errno   { return s.Errno }
func (s *PathOpenSyscall) private()       {}

type PathReadLinkSyscall struct {
	FD     FD
	Path   string
	Output []byte
	Errno  Errno
}

func (s *PathReadLinkSyscall) ID() SyscallID  { return PathReadLink }
func (s *PathReadLinkSyscall) Params() []any  { return []any{s.FD, s.Path} }
func (s *PathReadLinkSyscall) Results() []any { return []any{s.Output, s.Errno} }
func (s *PathReadLinkSyscall) Error() Errno   { return s.Errno }
func (s *PathReadLinkSyscall) private()       {}

type PathRemoveDirectorySyscall struct {
	FD    FD
	Path  string
	Errno Errno
}

func (s *PathRemoveDirectorySyscall) ID() SyscallID  { return PathRemoveDirectory }
func (s *PathRemoveDirectorySyscall) Params() []any  { return []any{s.FD, s.Path} }
func (s *PathRemoveDirectorySyscall) Results() []any { return []any{s.Errno} }
func (s *PathRemoveDirectorySyscall) Error() Errno   { return s.Errno }
func (s *PathRemoveDirectorySyscall) private()       {}

type PathRenameSyscall struct {
	FD      FD
	OldPath string
	NewFD   FD
	NewPath string
	Errno   Errno
}

func (s *PathRenameSyscall) ID() SyscallID  { return PathRename }
func (s *PathRenameSyscall) Params() []any  { return []any{s.FD, s.OldPath, s.NewFD, s.NewPath} }
func (s *PathRenameSyscall) Results() []any { return []any{s.Errno} }
func (s *PathRenameSyscall) Error() Errno   { return s.Errno }
func (s *PathRenameSyscall) private()       {}

type PathSymlinkSyscall struct {
	OldPath string
	FD      FD
	NewPath string
	Errno   Errno
}

func (s *PathSymlinkSyscall) ID() SyscallID  { return PathSymlink }
func (s *PathSymlinkSyscall) Params() []any  { return []any{s.OldPath, s.FD, s.NewPath} }
func (s *PathSymlinkSyscall) Results() []any { return []any{s.Errno} }
func (s *PathSymlinkSyscall) Error() Errno   { return s.Errno }
func (s *PathSymlinkSyscall) private()       {}

type PathUnlinkFileSyscall struct {
	FD    FD
	Path  string
	Errno Errno
}

func (s *PathUnlinkFileSyscall) ID() SyscallID  { return PathUnlinkFile }
func (s *PathUnlinkFileSyscall) Params() []any  { return []any{s.FD, s.Path} }
func (s *PathUnlinkFileSyscall) Results() []any { return []any{s.Errno} }
func (s *PathUnlinkFileSyscall) Error() Errno   { return s.Errno }
func (s *PathUnlinkFileSyscall) private()       {}

type PollOneOffSyscall struct {
	Subscriptions []Subscription
	Events        []Event
	Errno         Errno
}

func (s *PollOneOffSyscall) ID() SyscallID  { return PollOneOff }
func (s *PollOneOffSyscall) Params() []any  { return []any{s.Subscriptions, s.Events} }
func (s *PollOneOffSyscall) Results() []any { return []any{len(s.Events), s.Errno} }
func (s *PollOneOffSyscall) Error() Errno   { return s.Errno }
func (s *PollOneOffSyscall) private()       {}

type ProcExitSyscall struct {
	ExitCode ExitCode
	Errno    Errno
}

func (s *ProcExitSyscall) ID() SyscallID  { return ProcExit }
func (s *ProcExitSyscall) Params() []any  { return []any{s.ExitCode} }
func (s *ProcExitSyscall) Results() []any { return []any{s.Errno} }
func (s *ProcExitSyscall) Error() Errno   { return s.Errno }
func (s *ProcExitSyscall) private()       {}

type ProcRaiseSyscall struct {
	Signal Signal
	Errno  Errno
}

func (s *ProcRaiseSyscall) ID() SyscallID  { return ProcRaise }
func (s *ProcRaiseSyscall) Params() []any  { return []any{s.Signal} }
func (s *ProcRaiseSyscall) Results() []any { return []any{s.Errno} }
func (s *ProcRaiseSyscall) Error() Errno   { return s.Errno }
func (s *ProcRaiseSyscall) private()       {}

type SchedYieldSyscall struct {
	Errno Errno
}

func (s *SchedYieldSyscall) ID() SyscallID  { return SchedYield }
func (s *SchedYieldSyscall) Params() []any  { return nil }
func (s *SchedYieldSyscall) Results() []any { return []any{s.Errno} }
func (s *SchedYieldSyscall) Error() Errno   { return s.Errno }
func (s *SchedYieldSyscall) private()       {}

type RandomGetSyscall struct {
	B     []byte
	Errno Errno
}

func (s *RandomGetSyscall) ID() SyscallID  { return RandomGet }
func (s *RandomGetSyscall) Params() []any  { return []any{s.B} }
func (s *RandomGetSyscall) Results() []any { return []any{s.Errno} }
func (s *RandomGetSyscall) Error() Errno   { return s.Errno }
func (s *RandomGetSyscall) private()       {}

type SockAcceptSyscall struct {
	FD    FD
	Flags FDFlags
	NewFD FD
	Peer  SocketAddress
	Addr  SocketAddress
	Errno Errno
}

func (s *SockAcceptSyscall) ID() SyscallID  { return SockAccept }
func (s *SockAcceptSyscall) Params() []any  { return []any{s.FD, s.Flags} }
func (s *SockAcceptSyscall) Results() []any { return []any{s.NewFD, s.Peer, s.Addr, s.Errno} }
func (s *SockAcceptSyscall) Error() Errno   { return s.Errno }
func (s *SockAcceptSyscall) private()       {}

type SockShutdownSyscall struct {
	FD    FD
	Flags SDFlags
	Errno Errno
}

func (s *SockShutdownSyscall) ID() SyscallID  { return SockShutdown }
func (s *SockShutdownSyscall) Params() []any  { return []any{s.FD, s.Flags} }
func (s *SockShutdownSyscall) Results() []any { return []any{s.Errno} }
func (s *SockShutdownSyscall) Error() Errno   { return s.Errno }
func (s *SockShutdownSyscall) private()       {}

type SockRecvSyscall struct {
	FD     FD
	IOVecs []IOVec
	IFlags RIFlags
	Size   Size
	OFlags ROFlags
	Errno  Errno
}

func (s *SockRecvSyscall) ID() SyscallID  { return SockRecv }
func (s *SockRecvSyscall) Params() []any  { return []any{s.FD, s.IOVecs, s.IFlags} }
func (s *SockRecvSyscall) Results() []any { return []any{s.Size, s.OFlags, s.Errno} }
func (s *SockRecvSyscall) Error() Errno   { return s.Errno }
func (s *SockRecvSyscall) private()       {}

type SockSendSyscall struct {
	FD     FD
	IOVecs []IOVec
	IFlags SIFlags
	Size   Size
	Errno  Errno
}

func (s *SockSendSyscall) ID() SyscallID  { return SockSend }
func (s *SockSendSyscall) Params() []any  { return []any{s.FD, s.IOVecs, s.IFlags} }
func (s *SockSendSyscall) Results() []any { return []any{s.Size, s.Errno} }
func (s *SockSendSyscall) Error() Errno   { return s.Errno }
func (s *SockSendSyscall) private()       {}

type SockOpenSyscall struct {
	Family           ProtocolFamily
	SocketType       SocketType
	Protocol         Protocol
	RightsBase       Rights
	RightsInheriting Rights
	FD               FD
	Errno            Errno
}

func (s *SockOpenSyscall) ID() SyscallID { return SockOpen }
func (s *SockOpenSyscall) Params() []any {
	return []any{s.Family, s.SocketType, s.Protocol, s.RightsBase, s.RightsInheriting}
}
func (s *SockOpenSyscall) Results() []any { return []any{s.FD, s.Errno} }
func (s *SockOpenSyscall) Error() Errno   { return s.Errno }
func (s *SockOpenSyscall) private()       {}

type SockBindSyscall struct {
	FD    FD
	Bind  SocketAddress
	Addr  SocketAddress
	Errno Errno
}

func (s *SockBindSyscall) ID() SyscallID  { return SockBind }
func (s *SockBindSyscall) Params() []any  { return []any{s.FD, s.Bind} }
func (s *SockBindSyscall) Results() []any { return []any{s.Addr, s.Errno} }
func (s *SockBindSyscall) Error() Errno   { return s.Errno }
func (s *SockBindSyscall) private()       {}

type SockConnectSyscall struct {
	FD    FD
	Peer  SocketAddress
	Addr  SocketAddress
	Errno Errno
}

func (s *SockConnectSyscall) ID() SyscallID  { return SockConnect }
func (s *SockConnectSyscall) Params() []any  { return []any{s.FD, s.Peer} }
func (s *SockConnectSyscall) Results() []any { return []any{s.Addr, s.Errno} }
func (s *SockConnectSyscall) Error() Errno   { return s.Errno }
func (s *SockConnectSyscall) private()       {}

type SockListenSyscall struct {
	FD      FD
	Backlog int
	Errno   Errno
}

func (s *SockListenSyscall) ID() SyscallID  { return SockListen }
func (s *SockListenSyscall) Params() []any  { return []any{s.FD, s.Backlog} }
func (s *SockListenSyscall) Results() []any { return []any{s.Errno} }
func (s *SockListenSyscall) Error() Errno   { return s.Errno }
func (s *SockListenSyscall) private()       {}

type SockSendToSyscall struct {
	FD     FD
	IOVecs []IOVec
	IFlags SIFlags
	Addr   SocketAddress
	Size   Size
	Errno  Errno
}

func (s *SockSendToSyscall) ID() SyscallID  { return SockSendTo }
func (s *SockSendToSyscall) Params() []any  { return []any{s.FD, s.IOVecs, s.IFlags, s.Addr} }
func (s *SockSendToSyscall) Results() []any { return []any{s.Size, s.Errno} }
func (s *SockSendToSyscall) Error() Errno   { return s.Errno }
func (s *SockSendToSyscall) private()       {}

type SockRecvFromSyscall struct {
	FD     FD
	IOVecs []IOVec
	IFlags RIFlags
	Size   Size
	OFlags ROFlags
	Addr   SocketAddress
	Errno  Errno
}

func (s *SockRecvFromSyscall) ID() SyscallID  { return SockRecvFrom }
func (s *SockRecvFromSyscall) Params() []any  { return []any{s.FD, s.IOVecs, s.IFlags} }
func (s *SockRecvFromSyscall) Results() []any { return []any{s.Size, s.OFlags, s.Addr, s.Errno} }
func (s *SockRecvFromSyscall) Error() Errno   { return s.Errno }
func (s *SockRecvFromSyscall) private()       {}

type SockGetOptSyscall struct {
	FD     FD
	Option SocketOption
	Value  SocketOptionValue
	Errno  Errno
}

func (s *SockGetOptSyscall) ID() SyscallID  { return SockGetOpt }
func (s *SockGetOptSyscall) Params() []any  { return []any{s.FD, s.Option} }
func (s *SockGetOptSyscall) Results() []any { return []any{s.Value, s.Errno} }
func (s *SockGetOptSyscall) Error() Errno   { return s.Errno }
func (s *SockGetOptSyscall) private()       {}

type SockSetOptSyscall struct {
	FD     FD
	Option SocketOption
	Value  SocketOptionValue
	Errno  Errno
}

func (s *SockSetOptSyscall) ID() SyscallID  { return SockSetOpt }
func (s *SockSetOptSyscall) Params() []any  { return []any{s.FD, s.Option, s.Value} }
func (s *SockSetOptSyscall) Results() []any { return []any{s.Errno} }
func (s *SockSetOptSyscall) Error() Errno   { return s.Errno }
func (s *SockSetOptSyscall) private()       {}

type SockLocalAddressSyscall struct {
	FD    FD
	Addr  SocketAddress
	Errno Errno
}

func (s *SockLocalAddressSyscall) ID() SyscallID  { return SockLocalAddress }
func (s *SockLocalAddressSyscall) Params() []any  { return []any{s.FD} }
func (s *SockLocalAddressSyscall) Results() []any { return []any{s.Addr, s.Errno} }
func (s *SockLocalAddressSyscall) Error() Errno   { return s.Errno }
func (s *SockLocalAddressSyscall) private()       {}

type SockRemoteAddressSyscall struct {
	FD    FD
	Addr  SocketAddress
	Errno Errno
}

func (s *SockRemoteAddressSyscall) ID() SyscallID  { return SockRemoteAddress }
func (s *SockRemoteAddressSyscall) Params() []any  { return []any{s.FD} }
func (s *SockRemoteAddressSyscall) Results() []any { return []any{s.Addr, s.Errno} }
func (s *SockRemoteAddressSyscall) Error() Errno   { return s.Errno }
func (s *SockRemoteAddressSyscall) private()       {}

type SockAddressInfoSyscall struct {
	Name    string
	Service string
	Hints   AddressInfo
	Res     []AddressInfo
	Errno   Errno
}

func (s *SockAddressInfoSyscall) ID() SyscallID  { return SockAddressInfo }
func (s *SockAddressInfoSyscall) Params() []any  { return []any{s.Name, s.Service, s.Hints} }
func (s *SockAddressInfoSyscall) Results() []any { return []any{s.Res, s.Errno} }
func (s *SockAddressInfoSyscall) Error() Errno   { return s.Errno }
func (s *SockAddressInfoSyscall) private()       {}

// SyscallID is a system call identifier.
type SyscallID int

const (
	ArgsSizesGet SyscallID = iota
	ArgsGet
	EnvironSizesGet
	EnvironGet
	ClockResGet
	ClockTimeGet
	FDAdvise
	FDAllocate
	FDClose
	FDDataSync
	FDStatGet
	FDStatSetFlags
	FDStatSetRights
	FDFileStatGet
	FDFileStatSetSize
	FDFileStatSetTimes
	FDPread
	FDPreStatGet
	FDPreStatDirName
	FDPwrite
	FDRead
	FDReadDir
	FDRenumber
	FDSeek
	FDSync
	FDTell
	FDWrite
	PathCreateDirectory
	PathFileStatGet
	PathFileStatSetTimes
	PathLink
	PathOpen
	PathReadLink
	PathRemoveDirectory
	PathRename
	PathSymlink
	PathUnlinkFile
	PollOneOff
	ProcExit
	ProcRaise
	SchedYield
	RandomGet
	SockAccept
	SockRecv
	SockSend
	SockShutdown

	SockOpen
	SockBind
	SockConnect
	SockListen
	SockSendTo
	SockRecvFrom
	SockGetOpt
	SockSetOpt
	SockLocalAddress
	SockRemoteAddress
	SockAddressInfo
)

func (s SyscallID) String() string {
	if int(s) >= len(syscallIDStrings) {
		return fmt.Sprintf("SyscallID(%d)", int(s))
	}
	return syscallIDStrings[s]
}

var syscallIDStrings = [...]string{
	"ArgsSizesGet",
	"ArgsGet",
	"EnvironSizesGet",
	"EnvironGet",
	"ClockResGet",
	"ClockTimeGet",
	"FDAdvise",
	"FDAllocate",
	"FDClose",
	"FDDataSync",
	"FDStatGet",
	"FDStatSetFlags",
	"FDStatSetRights",
	"FDFileStatGet",
	"FDFileStatSetSize",
	"FDFileStatSetTimes",
	"FDPread",
	"FDPreStatGet",
	"FDPreStatDirName",
	"FDPwrite",
	"FDRead",
	"FDReadDir",
	"FDRenumber",
	"FDSeek",
	"FDSync",
	"FDTell",
	"FDWrite",
	"PathCreateDirectory",
	"PathFileStatGet",
	"PathFileStatSetTimes",
	"PathLink",
	"PathOpen",
	"PathReadLink",
	"PathRemoveDirectory",
	"PathRename",
	"PathSymlink",
	"PathUnlinkFile",
	"PollOneOff",
	"ProcExit",
	"ProcRaise",
	"SchedYield",
	"RandomGet",
	"SockAccept",
	"SockRecv",
	"SockSend",
	"SockShutdown",
	"SockOpen",
	"SockBind",
	"SockConnect",
	"SockListen",
	"SockSendTo",
	"SockRecvFrom",
	"SockGetOpt",
	"SockSetOpt",
	"SockLocalAddress",
	"SockRemoteAddress",
	"SockAddressInfo",
}
