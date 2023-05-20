package wasicall

import (
	. "github.com/stealthrocket/wasi-go"
)

type Encoder struct{}

func (e *Encoder) ArgsGet(args []string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) EnvironGet(args []string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) ClockResGet(id ClockID, timestamp Timestamp, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) ClockTimeGet(id ClockID, precision Timestamp, timestamp Timestamp, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDAdvise(fd FD, offset FileSize, length FileSize, advice Advice, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDAllocate(fd FD, offset FileSize, length FileSize, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDClose(fd FD, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDDataSync(fd FD, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDStatGet(fd FD, stat FDStat, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDStatSetFlags(fd FD, flags FDFlags, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDStatSetRights(fd FD, rightsBase, rightsInheriting Rights, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDFileStatGet(fd FD, stat FileStat, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDFileStatSetSize(fd FD, size FileSize, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDFileStatSetTimes(fd FD, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDPread(fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDPreStatGet(fd FD, stat PreStat, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDPreStatDirName(fd FD, name string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDPwrite(fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDRead(fd FD, iovecs []IOVec, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDReadDir(fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int, count int, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDRenumber(from, to FD, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDSeek(fd FD, seekOffset FileDelta, whence Whence, offset FileSize, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDSync(fd FD, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDTell(fd FD, offset FileSize, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) FDWrite(fd FD, iovecs []IOVec, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathCreateDirectory(fd FD, path string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathFileStatGet(fd FD, lookupFlags LookupFlags, path string, fileStat FileStat, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathFileStatSetTimes(fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathLink(oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathOpen(fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags, newfd FD, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathReadLink(fd FD, path string, buffer []byte, output []byte, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathRemoveDirectory(fd FD, path string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathRename(fd FD, oldPath string, newFD FD, newPath string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathSymlink(oldPath string, fd FD, newPath string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PathUnlinkFile(fd FD, path string, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) PollOneOff(subscriptions []Subscription, events []Event, n int, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) ProcExit(exitCode ExitCode, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) ProcRaise(signal Signal, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SchedYield(errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) RandomGet(b []byte, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockAccept(fd FD, flags FDFlags, newfd FD, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockRecv(fd FD, iovecs []IOVec, flags RIFlags, size Size, oflags ROFlags, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockSend(fd FD, iovecs []IOVec, flags SIFlags, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockShutdown(fd FD, flags SDFlags, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockOpen(family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights, fd FD, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockBind(fd FD, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockConnect(fd FD, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockListen(fd FD, backlog int, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockSendTo(fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockRecvFrom(fd FD, iovecs []IOVec, iflags RIFlags, size Size, oflags ROFlags, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockGetOptInt(fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockSetOptInt(fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockLocalAddress(fd FD, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (e *Encoder) SockPeerAddress(fd FD, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}
