package wasicall

type Syscall int

const (
	// Preview 1 system calls.
	ArgsGet Syscall = iota
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

	// Sockets extension system calls.
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
