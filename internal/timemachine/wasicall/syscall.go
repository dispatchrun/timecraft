package wasicall

import (
	"fmt"

	"github.com/stealthrocket/wasi-go"
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

	// Results it the set of return values.
	Results() []any

	// Errno is the system call error.
	Errno() wasi.Errno

	private()
}

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
