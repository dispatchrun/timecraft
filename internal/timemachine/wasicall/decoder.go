package wasicall

import (
	. "github.com/stealthrocket/wasi-go"
)

type Decoder struct{}

func (d *Decoder) ArgsGet(b []byte) (args []string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) EnvironGet(b []byte) (args []string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) ClockResGet(b []byte) (id ClockID, timestamp Timestamp, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) ClockTimeGet(b []byte) (id ClockID, precision Timestamp, timestamp Timestamp, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDAdvise(b []byte) (fd FD, offset FileSize, length FileSize, advice Advice, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDAllocate(b []byte) (fd FD, offset FileSize, length FileSize, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDClose(b []byte) (fd FD, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDDataSync(b []byte) (fd FD, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDStatGet(b []byte) (fd FD, stat FDStat, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDStatSetFlags(b []byte) (fd FD, flags FDFlags, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDStatSetRights(b []byte) (fd FD, rightsBase, rightsInheriting Rights, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDFileStatGet(b []byte) (fd FD, stat FileStat, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDFileStatSetSize(b []byte) (fd FD, size FileSize, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDFileStatSetTimes(b []byte) (fd FD, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDPread(b []byte) (fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDPreStatGet(b []byte) (fd FD, stat PreStat, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDPreStatDirName(b []byte) (fd FD, name string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDPwrite(b []byte) (fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDRead(b []byte) (fd FD, iovecs []IOVec, size Size, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDReadDir(b []byte) (fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int, count int, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDRenumber(b []byte) (from, to FD, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDSeek(b []byte) (fd FD, seekOffset FileDelta, whence Whence, offset FileSize, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDSync(b []byte) (fd FD, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDTell(b []byte) (fd FD, offset FileSize, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) FDWrite(b []byte) (fd FD, iovecs []IOVec, size Size, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathCreateDirectory(b []byte) (fd FD, path string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathFileStatGet(b []byte) (fd FD, lookupFlags LookupFlags, path string, fileStat FileStat, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathFileStatSetTimes(b []byte) (fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathLink(b []byte) (oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathOpen(b []byte) (fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags, newfd FD, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathReadLink(b []byte) (fd FD, path string, buffer []byte, output []byte, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathRemoveDirectory(b []byte) (fd FD, path string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathRename(b []byte) (fd FD, oldPath string, newFD FD, newPath string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathSymlink(b []byte) (oldPath string, fd FD, newPath string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PathUnlinkFile(b []byte) (fd FD, path string, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) PollOneOff(b []byte) (subscriptions []Subscription, events []Event, n int, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) ProcExit(b []byte) (exitCode ExitCode, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) ProcRaise(b []byte) (signal Signal, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SchedYield(b []byte) (errno Errno) {
	panic("not implemented")
}

func (d *Decoder) RandomGet(b []byte) (buf []byte, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockAccept(b []byte) (fd FD, flags FDFlags, newfd FD, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockRecv(b []byte) (fd FD, iovecs []IOVec, flags RIFlags, size Size, oflags ROFlags, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockSend(b []byte) (fd FD, iovecs []IOVec, flags SIFlags, size Size, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockShutdown(b []byte) (fd FD, flags SDFlags, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockOpen(b []byte) (family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights, fd FD, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockBind(b []byte) (fd FD, addr SocketAddress, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockConnect(b []byte) (fd FD, addr SocketAddress, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockListen(b []byte) (fd FD, backlog int, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockSendTo(b []byte) (fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress, size Size, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockRecvFrom(b []byte) (fd FD, iovecs []IOVec, iflags RIFlags, size Size, oflags ROFlags, addr SocketAddress, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockGetOptInt(b []byte) (fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockSetOptInt(b []byte) (fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockLocalAddress(b []byte) (fd FD, addr SocketAddress, errno Errno) {
	panic("not implemented")
}

func (d *Decoder) SockPeerAddress(b []byte) (fd FD, addr SocketAddress, errno Errno) {
	panic("not implemented")
}
