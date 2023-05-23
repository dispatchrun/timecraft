package wasicall

import (
	"fmt"
)

type SyscallNumber int

const (
	ArgsSizesGet SyscallNumber = iota
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

func (s SyscallNumber) String() string {
	if int(s) >= len(syscallNumberStrings) {
		return fmt.Sprintf("SyscallNumber(%d)", int(s))
	}
	return syscallNumberStrings[s]
}

var syscallNumberStrings = [...]string{
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
