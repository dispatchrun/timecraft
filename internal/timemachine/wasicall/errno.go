package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
)

// NewErrnoSystem constructs a WASI System which returns an errno
// for all calls.
func NewErrnoSystem(errno Errno) System {
	return errnoSystem(errno)
}

type errnoSystem Errno

func (e errnoSystem) ArgsSizesGet(ctx context.Context) (argCount int, stringBytes int, errno Errno) {
	return 0, 0, Errno(e)
}

func (e errnoSystem) ArgsGet(ctx context.Context) ([]string, Errno) {
	return nil, Errno(e)
}

func (e errnoSystem) EnvironSizesGet(ctx context.Context) (argCount int, stringBytes int, errno Errno) {
	return 0, 0, Errno(e)
}

func (e errnoSystem) EnvironGet(ctx context.Context) ([]string, Errno) {
	return nil, Errno(e)
}

func (e errnoSystem) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	return Errno(e)
}

func (e errnoSystem) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	return Errno(e)
}

func (e errnoSystem) FDClose(ctx context.Context, fd FD) Errno {
	return Errno(e)
}

func (e errnoSystem) FDDataSync(ctx context.Context, fd FD) Errno {
	return Errno(e)
}

func (e errnoSystem) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	return FDStat{}, Errno(e)
}

func (e errnoSystem) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	return Errno(e)
}

func (e errnoSystem) FDStatSetRights(ctx context.Context, fd FD, rightsBase, rightsInheriting Rights) Errno {
	return Errno(e)
}

func (e errnoSystem) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	return FileStat{}, Errno(e)
}

func (e errnoSystem) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	return Errno(e)
}

func (e errnoSystem) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	return Errno(e)
}

func (e errnoSystem) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	return PreStat{}, Errno(e)
}

func (e errnoSystem) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	return "", Errno(e)
}

func (e errnoSystem) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) FDRenumber(ctx context.Context, from, to FD) Errno {
	return Errno(e)
}

func (e errnoSystem) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) FDSync(ctx context.Context, fd FD) Errno {
	return Errno(e)
}

func (e errnoSystem) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	return Errno(e)
}

func (e errnoSystem) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	return FileStat{}, Errno(e)
}

func (e errnoSystem) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	return Errno(e)
}

func (e errnoSystem) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	return Errno(e)
}

func (e errnoSystem) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	return -1, Errno(e)
}

func (e errnoSystem) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) (int, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	return Errno(e)
}

func (e errnoSystem) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	return Errno(e)
}

func (e errnoSystem) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	return Errno(e)
}

func (e errnoSystem) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	return Errno(e)
}

func (e errnoSystem) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	return Errno(e)
}

func (e errnoSystem) ProcRaise(ctx context.Context, signal Signal) Errno {
	return Errno(e)
}

func (e errnoSystem) SchedYield(ctx context.Context) Errno {
	return Errno(e)
}

func (e errnoSystem) RandomGet(ctx context.Context, b []byte) Errno {
	return Errno(e)
}

func (e errnoSystem) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, SocketAddress, SocketAddress, Errno) {
	return -1, nil, nil, Errno(e)
}

func (e errnoSystem) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, flags RIFlags) (Size, ROFlags, Errno) {
	return 0, 0, Errno(e)
}

func (e errnoSystem) SockSend(ctx context.Context, fd FD, iovecs []IOVec, flags SIFlags) (Size, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	return Errno(e)
}

func (e errnoSystem) Close(ctx context.Context) error {
	return nil
}

func (e errnoSystem) SockOpen(ctx context.Context, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights) (FD, Errno) {
	return -1, Errno(e)
}

func (e errnoSystem) SockBind(ctx context.Context, fd FD, addr SocketAddress) (SocketAddress, Errno) {
	return nil, Errno(e)
}

func (e errnoSystem) SockConnect(ctx context.Context, fd FD, addr SocketAddress) (SocketAddress, Errno) {
	return nil, Errno(e)
}

func (e errnoSystem) SockListen(ctx context.Context, fd FD, backlog int) Errno {
	return Errno(e)
}

func (e errnoSystem) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, flags SIFlags, addr SocketAddress) (Size, Errno) {
	return 0, Errno(e)
}

func (e errnoSystem) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, flags RIFlags) (Size, ROFlags, SocketAddress, Errno) {
	return 0, 0, nil, Errno(e)
}

func (e errnoSystem) SockGetOpt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (SocketOptionValue, Errno) {
	return nil, Errno(e)
}

func (e errnoSystem) SockSetOpt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value SocketOptionValue) Errno {
	return Errno(e)
}

func (e errnoSystem) SockLocalAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	return nil, Errno(e)
}

func (e errnoSystem) SockRemoteAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	return nil, Errno(e)
}

func (e errnoSystem) SockAddressInfo(ctx context.Context, name, service string, hints AddressInfo, results []AddressInfo) (int, Errno) {
	return 0, Errno(e)
}
