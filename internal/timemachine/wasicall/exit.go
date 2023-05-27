package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero/sys"
)

// NewExitSystem constructs a WASI System which exits all calls with the given
// exit code.
func NewExitSystem(exitCode int) System {
	return &exitSystem{exitCode: exitCode}
}

type exitSystem struct {
	exitCode int
}

func (s *exitSystem) newExitError() *sys.ExitError {
	return sys.NewExitError(uint32(s.exitCode))
}

func (s *exitSystem) Close(ctx context.Context) error {
	return nil
}

func (s *exitSystem) Preopen(hostfd int, path string, fdstat FDStat) FD {
	return -1
}

func (s *exitSystem) Register(hostfd int, fdstat FDStat) FD {
	return -1
}

func (s *exitSystem) ArgsSizesGet(ctx context.Context) (int, int, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) ArgsGet(ctx context.Context) ([]string, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) EnvironSizesGet(ctx context.Context) (int, int, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) EnvironGet(ctx context.Context) ([]string, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDClose(ctx context.Context, fd FD) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDDataSync(ctx context.Context, fd FD) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDStatSetRights(ctx context.Context, fd FD, rightsBase, rightsInheriting Rights) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDRenumber(ctx context.Context, from, to FD) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDSync(ctx context.Context, fd FD) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) ([]byte, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) ProcRaise(ctx context.Context, signal Signal) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) SchedYield(ctx context.Context) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) RandomGet(ctx context.Context, b []byte) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockSend(ctx context.Context, fd FD, iovecs []IOVec, flags SIFlags) (Size, Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	panic(s.newExitError())
}

func (s *exitSystem) SockOpen(ctx context.Context, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights) (newfd FD, errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockBind(ctx context.Context, fd FD, addr SocketAddress) (errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockConnect(ctx context.Context, fd FD, addr SocketAddress) (errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockListen(ctx context.Context, fd FD, backlog int) (errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, flags SIFlags, addr SocketAddress) (size Size, errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, flags RIFlags) (size Size, oflags ROFlags, addr SocketAddress, errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockGetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (value int, errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockSetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value int) (errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockLocalAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	panic(s.newExitError())
}

func (s *exitSystem) SockPeerAddress(ctx context.Context, fd FD) (addr SocketAddress, errno Errno) {
	panic(s.newExitError())
}

var (
	_ SocketsExtension = (*exitSystem)(nil)
)
