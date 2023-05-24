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
	SockGetOptInt
	SockSetOptInt
	SockLocalAddress
	SockPeerAddress
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
	"SockGetOptInt",
	"SockSetOptInt",
	"SockLocalAddress",
	"SockPeerAddress",
}
