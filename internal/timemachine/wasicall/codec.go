package wasicall

import (
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"

	. "github.com/stealthrocket/wasi-go"
)

// Codec is responsible for encoding and decoding system call inputs
// and outputs.
//
// The system calls are sealed so there's no need for forwards or backwards
// compatibility. Rather than use protobuf or flatbuffers or similar, we use
// simple bespoke encoders. We aren't too concerned with succinctness since
// the records are ultimately compressed, but it should be easy to experiment
// with different encodings (e.g. varints) by changing the append/read helpers.
// There are also other ways to reduce the size of encoded records, for
// example avoiding storing return values other than errno when errno!=0.
type Codec struct{}

func (c *Codec) EncodeArgsGet(buffer []byte, args []string, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	return appendStrings(buffer, args)
}

func (c *Codec) DecodeArgsGet(buffer []byte, args []string) (_ []string, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	args, buffer, err = readStrings(buffer, args)
	return args, errno, err
}

func (c *Codec) EncodeEnvironGet(buffer []byte, env []string, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	return appendStrings(buffer, env)
}

func (c *Codec) DecodeEnvironGet(buffer []byte, env []string) (_ []string, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	env, buffer, err = readStrings(buffer, env)
	return env, errno, err
}

func (c *Codec) EncodeClockResGet(buffer []byte, id ClockID, timestamp Timestamp, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendClockID(buffer, id)
	return appendTimestamp(buffer, timestamp)
}

func (c *Codec) DecodeClockResGet(buffer []byte) (id ClockID, timestamp Timestamp, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if id, buffer, err = readClockID(buffer); err != nil {
		return
	}
	timestamp, buffer, err = readTimestamp(buffer)
	return
}

func (c *Codec) EncodeClockTimeGet(buffer []byte, id ClockID, precision Timestamp, timestamp Timestamp, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendClockID(buffer, id)
	buffer = appendTimestamp(buffer, precision)
	return appendTimestamp(buffer, timestamp)
}

func (c *Codec) DecodeClockTimeGet(buffer []byte) (id ClockID, precision Timestamp, timestamp Timestamp, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if id, buffer, err = readClockID(buffer); err != nil {
		return
	}
	if precision, buffer, err = readTimestamp(buffer); err != nil {
		return
	}
	timestamp, buffer, err = readTimestamp(buffer)
	return
}

func (c *Codec) EncodeFDAdvise(buffer []byte, fd FD, offset FileSize, length FileSize, advice Advice, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	buffer = appendFileSize(buffer, offset)
	buffer = appendFileSize(buffer, length)
	return appendAdvice(buffer, advice)
}

func (c *Codec) DecodeFDAdvise(buffer []byte) (fd FD, offset FileSize, length FileSize, advice Advice, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if offset, buffer, err = readFileSize(buffer); err != nil {
		return
	}
	if length, buffer, err = readFileSize(buffer); err != nil {
		return
	}
	advice, buffer, err = readAdvice(buffer)
	return
}

func (c *Codec) EncodeFDAllocate(buffer []byte, fd FD, offset FileSize, length FileSize, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	buffer = appendFileSize(buffer, offset)
	return appendFileSize(buffer, length)
}

func (c *Codec) DecodeFDAllocate(buffer []byte) (fd FD, offset FileSize, length FileSize, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if offset, buffer, err = readFileSize(buffer); err != nil {
		return
	}
	length, buffer, err = readFileSize(buffer)
	return
}

func (c *Codec) EncodeFDClose(buffer []byte, fd FD, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	return appendFD(buffer, fd)
}

func (c *Codec) DecodeFDClose(buffer []byte) (fd FD, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	fd, buffer, err = readFD(buffer)
	return
}

func (c *Codec) EncodeFDDataSync(buffer []byte, fd FD, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	return appendFD(buffer, fd)
}

func (c *Codec) DecodeFDDataSync(buffer []byte) (fd FD, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	fd, buffer, err = readFD(buffer)
	return
}

func (c *Codec) EncodeFDStatGet(buffer []byte, fd FD, stat FDStat, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	return appendFDStat(buffer, stat)
}

func (c *Codec) DecodeFDStatGet(buffer []byte) (fd FD, stat FDStat, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	stat, buffer, err = readFDStat(buffer)
	return
}

func (c *Codec) EncodeFDStatSetFlags(buffer []byte, fd FD, flags FDFlags, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	return appendFDFlags(buffer, flags)
}

func (c *Codec) DecodeFDStatSetFlags(buffer []byte) (fd FD, flags FDFlags, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	flags, buffer, err = readFDFlags(buffer)
	return
}

func (c *Codec) EncodeFDStatSetRights(buffer []byte, fd FD, rightsBase, rightsInheriting Rights, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	buffer = appendRights(buffer, rightsBase)
	return appendRights(buffer, rightsInheriting)
}

func (c *Codec) DecodeFDStatSetRights(buffer []byte) (fd FD, rightsBase, rightsInheriting Rights, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if rightsBase, buffer, err = readRights(buffer); err != nil {
		return
	}
	rightsInheriting, buffer, err = readRights(buffer)
	return
}

func (c *Codec) EncodeFDFileStatGet(buffer []byte, fd FD, stat FileStat, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	return appendFileStat(buffer, stat)
}

func (c *Codec) DecodeFDFileStatGet(buffer []byte) (fd FD, stat FileStat, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	stat, buffer, err = readFileStat(buffer)
	return
}

func (c *Codec) EncodeFDFileStatSetSize(buffer []byte, fd FD, size FileSize, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	return appendFileSize(buffer, size)
}

func (c *Codec) DecodeFDFileStatSetSize(buffer []byte) (fd FD, size FileSize, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	size, buffer, err = readFileSize(buffer)
	return
}

func (c *Codec) EncodeFDFileStatSetTimes(buffer []byte, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	buffer = appendTimestamp(buffer, accessTime)
	buffer = appendTimestamp(buffer, modifyTime)
	return appendFSTFlags(buffer, flags)
}

func (c *Codec) DecodeFDFileStatSetTimes(buffer []byte) (fd FD, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if accessTime, buffer, err = readTimestamp(buffer); err != nil {
		return
	}
	if modifyTime, buffer, err = readTimestamp(buffer); err != nil {
		return
	}
	flags, buffer, err = readFSTFlags(buffer)
	return
}

func (c *Codec) EncodeFDPread(buffer []byte, fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDPread(buffer []byte) (fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDPreStatGet(buffer []byte, fd FD, stat PreStat, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	return appendPreStat(buffer, stat)
}

func (c *Codec) DecodeFDPreStatGet(buffer []byte) (fd FD, stat PreStat, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	stat, buffer, err = readPreStat(buffer)
	return
}

func (c *Codec) EncodeFDPreStatDirName(buffer []byte, fd FD, name string, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDPreStatDirName(buffer []byte) (fd FD, name string, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDPwrite(buffer []byte, fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDPwrite(buffer []byte) (fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDRead(buffer []byte, fd FD, iovecs []IOVec, size Size, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	buffer = appendIOVecs(buffer, iovecs)
	return appendSize(buffer, size)
}

func (c *Codec) DecodeFDRead(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, size Size, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = readIOVecs(buffer, iovecs); err != nil {
		return
	}
	size, buffer, err = readSize(buffer)
	return fd, iovecs, size, errno, err
}

func (c *Codec) EncodeFDReadDir(buffer []byte, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int, count int, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDReadDir(buffer []byte) (fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int, count int, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDRenumber(buffer []byte, from, to FD, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDRenumber(buffer []byte) (from, to FD, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDSeek(buffer []byte, fd FD, seekOffset FileDelta, whence Whence, offset FileSize, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDSeek(buffer []byte) (fd FD, seekOffset FileDelta, whence Whence, offset FileSize, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDSync(buffer []byte, fd FD, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	return appendFD(buffer, fd)
}

func (c *Codec) DecodeFDSync(buffer []byte) (fd FD, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	fd, buffer, err = readFD(buffer)
	return
}

func (c *Codec) EncodeFDTell(buffer []byte, fd FD, offset FileSize, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDTell(buffer []byte) (fd FD, offset FileSize, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDWrite(buffer []byte, fd FD, iovecs []IOVec, size Size, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	buffer = appendIOVecs(buffer, iovecs)
	return appendSize(buffer, size)
}

func (c *Codec) DecodeFDWrite(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, size Size, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = readIOVecs(buffer, iovecs); err != nil {
		return
	}
	size, buffer, err = readSize(buffer)
	return fd, iovecs, size, errno, err
}

func (c *Codec) EncodePathCreateDirectory(buffer []byte, fd FD, path string, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathCreateDirectory(buffer []byte) (fd FD, path string, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathFileStatGet(buffer []byte, fd FD, lookupFlags LookupFlags, path string, fileStat FileStat, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathFileStatGet(buffer []byte) (fd FD, lookupFlags LookupFlags, path string, fileStat FileStat, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathFileStatSetTimes(buffer []byte, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathFileStatSetTimes(buffer []byte) (fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathLink(buffer []byte, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathLink(buffer []byte) (oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathOpen(buffer []byte, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags, newfd FD, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathOpen(buffer []byte) (fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags, newfd FD, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathReadLink(buffer []byte, fd FD, path string, b []byte, output []byte, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathReadLink(buffer []byte) (fd FD, path string, b []byte, output []byte, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathRemoveDirectory(buffer []byte, fd FD, path string, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathRemoveDirectory(buffer []byte) (fd FD, path string, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathRename(buffer []byte, fd FD, oldPath string, newFD FD, newPath string, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathRename(buffer []byte) (fd FD, oldPath string, newFD FD, newPath string, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathSymlink(buffer []byte, oldPath string, fd FD, newPath string, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathSymlink(buffer []byte) (oldPath string, fd FD, newPath string, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePathUnlinkFile(buffer []byte, fd FD, path string, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodePathUnlinkFile(buffer []byte) (fd FD, path string, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodePollOneOff(buffer []byte, subscriptions []Subscription, events []Event, n int, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendSubscriptions(buffer, subscriptions)
	buffer = appendEvents(buffer, events)
	return appendInt(buffer, n)
}

func (c *Codec) DecodePollOneOff(buffer []byte, subscriptions []Subscription, events []Event) (_ []Subscription, _ []Event, n int, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if subscriptions, buffer, err = readSubscriptions(buffer, subscriptions); err != nil {
		return
	}
	if events, buffer, err = readEvents(buffer, events); err != nil {
		return
	}
	n, buffer, err = readInt(buffer)
	return subscriptions, events, n, errno, err
}

func (c *Codec) EncodeProcExit(buffer []byte, exitCode ExitCode, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	return appendExitCode(buffer, exitCode)
}

func (c *Codec) DecodeProcExit(buffer []byte) (exitCode ExitCode, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	exitCode, buffer, err = readExitCode(buffer)
	return
}

func (c *Codec) EncodeProcRaise(buffer []byte, signal Signal, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	return appendSignal(buffer, signal)
}

func (c *Codec) DecodeProcRaise(buffer []byte) (signal Signal, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	signal, buffer, err = readSignal(buffer)
	return
}

func (c *Codec) EncodeSchedYield(buffer []byte, errno Errno) []byte {
	return appendErrno(buffer, errno)
}

func (c *Codec) DecodeSchedYield(buffer []byte) (errno Errno, err error) {
	errno, buffer, err = readErrno(buffer)
	return
}

func (c *Codec) EncodeRandomGet(buffer []byte, b []byte, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendBytes(buffer, b)
	return buffer
}

func (c *Codec) DecodeRandomGet(buffer []byte) (result []byte, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	result, buffer, err = readBytes(buffer)
	return
}

func (c *Codec) EncodeSockAccept(buffer []byte, fd FD, flags FDFlags, newfd FD, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	buffer = appendFDFlags(buffer, flags)
	return appendFD(buffer, newfd)
}

func (c *Codec) DecodeSockAccept(buffer []byte) (fd FD, flags FDFlags, newfd FD, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if flags, buffer, err = readFDFlags(buffer); err != nil {
		return
	}
	newfd, buffer, err = readFD(buffer)
	return
}

func (c *Codec) EncodeSockRecv(buffer []byte, fd FD, iovecs []IOVec, flags RIFlags, size Size, oflags ROFlags, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockRecv(buffer []byte) (fd FD, iovecs []IOVec, flags RIFlags, size Size, oflags ROFlags, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockSend(buffer []byte, fd FD, iovecs []IOVec, flags SIFlags, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockSend(buffer []byte) (fd FD, iovecs []IOVec, flags SIFlags, size Size, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockShutdown(buffer []byte, fd FD, flags SDFlags, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockShutdown(buffer []byte) (fd FD, flags SDFlags, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockOpen(buffer []byte, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights, fd FD, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockOpen(buffer []byte) (family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights, fd FD, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockBind(buffer []byte, fd FD, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockBind(buffer []byte) (fd FD, addr SocketAddress, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockConnect(buffer []byte, fd FD, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockConnect(buffer []byte) (fd FD, addr SocketAddress, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockListen(buffer []byte, fd FD, backlog int, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockListen(buffer []byte) (fd FD, backlog int, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockSendTo(buffer []byte, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress, size Size, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockSendTo(buffer []byte) (fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress, size Size, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockRecvFrom(buffer []byte, fd FD, iovecs []IOVec, iflags RIFlags, size Size, oflags ROFlags, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockRecvFrom(buffer []byte) (fd FD, iovecs []IOVec, iflags RIFlags, size Size, oflags ROFlags, addr SocketAddress, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockGetOptInt(buffer []byte, fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockGetOptInt(buffer []byte) (fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockSetOptInt(buffer []byte, fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockSetOptInt(buffer []byte) (fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockLocalAddress(buffer []byte, fd FD, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockLocalAddress(buffer []byte) (fd FD, addr SocketAddress, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSockPeerAddress(buffer []byte, fd FD, addr SocketAddress, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSockPeerAddress(buffer []byte) (fd FD, addr SocketAddress, errno Errno, err error) {
	panic("not implemented")
}

func appendU32(b []byte, v uint32) []byte {
	return binary.LittleEndian.AppendUint32(b, v)
}

func readU32(b []byte) (uint32, []byte, error) {
	if len(b) < 4 {
		return 0, nil, io.ErrShortBuffer
	}
	return binary.LittleEndian.Uint32(b), b[4:], nil
}

func appendU64(b []byte, v uint64) []byte {
	return binary.LittleEndian.AppendUint64(b, v)
}

func readU64(b []byte) (uint64, []byte, error) {
	if len(b) < 8 {
		return 0, nil, io.ErrShortBuffer
	}
	return binary.LittleEndian.Uint64(b), b[8:], nil
}

func appendInt(b []byte, v int) []byte {
	return appendU32(b, uint32(v))
}

func readInt(b []byte) (int, []byte, error) {
	v, b, err := readU32(b)
	return int(v), b, err
}

func appendBytes(buffer []byte, b []byte) []byte {
	buffer = appendU32(buffer, uint32(len(b)))
	return append(buffer, b...)
}

func readBytes(buffer []byte) ([]byte, []byte, error) {
	length, buffer, err := readU32(buffer)
	if err != nil {
		return nil, buffer, err
	}
	if uint32(len(buffer)) < length {
		return nil, buffer, io.ErrShortBuffer
	}
	return buffer[:length], buffer[length:], err
}

func appendString(buffer []byte, s string) []byte {
	return appendBytes(buffer, unsafe.Slice(unsafe.StringData(s), len(s)))
}

func readString(buffer []byte) (string, []byte, error) {
	result, buffer, err := readBytes(buffer)
	if err != nil || len(result) == 0 {
		return "", buffer, err
	}
	return unsafe.String(&result[0], len(result)), buffer, err
}

func appendStrings(buffer []byte, args []string) []byte {
	buffer = appendU32(buffer, uint32(len(args)))
	for _, arg := range args {
		buffer = appendString(buffer, arg)
	}
	return buffer
}

func readStrings(buffer []byte, strings []string) (_ []string, _ []byte, err error) {
	var count uint32
	if count, buffer, err = readU32(buffer); err != nil {
		return
	}
	if uint32(len(strings)) < count {
		strings = make([]string, count)
	} else {
		strings = strings[:count]
	}
	for i := uint32(0); i < count; i++ {
		strings[i], buffer, err = readString(buffer)
		if err != nil {
			return
		}
	}
	return strings, buffer, nil
}

func appendIOVecs(buffer []byte, iovecs []IOVec) []byte {
	buffer = appendU32(buffer, uint32(len(iovecs)))
	for _, iovec := range iovecs {
		buffer = appendBytes(buffer, iovec)
	}
	return buffer
}

func readIOVecs(buffer []byte, iovecs []IOVec) (_ []IOVec, _ []byte, err error) {
	var count uint32
	if count, buffer, err = readU32(buffer); err != nil {
		return
	}
	if uint32(len(iovecs)) < count {
		iovecs = make([]IOVec, count)
	} else {
		iovecs = iovecs[:count]
	}
	for i := uint32(0); i < count; i++ {
		iovecs[i], buffer, err = readBytes(buffer)
		if err != nil {
			return
		}
	}
	return iovecs, buffer, nil
}

func appendErrno(buffer []byte, errno Errno) []byte {
	return appendU32(buffer, uint32(errno))
}

func readErrno(buffer []byte) (Errno, []byte, error) {
	errno, buffer, err := readU32(buffer)
	return Errno(errno), buffer, err
}

func appendFD(buffer []byte, fd FD) []byte {
	return appendU32(buffer, uint32(fd))
}

func readFD(buffer []byte) (FD, []byte, error) {
	fd, buffer, err := readU32(buffer)
	return FD(fd), buffer, err
}

func appendClockID(buffer []byte, id ClockID) []byte {
	return appendU32(buffer, uint32(id))
}

func readClockID(buffer []byte) (ClockID, []byte, error) {
	id, buffer, err := readU32(buffer)
	return ClockID(id), buffer, err
}

func appendTimestamp(buffer []byte, id Timestamp) []byte {
	return appendU64(buffer, uint64(id))
}

func readTimestamp(buffer []byte) (Timestamp, []byte, error) {
	id, buffer, err := readU64(buffer)
	return Timestamp(id), buffer, err
}

func appendSize(buffer []byte, id Size) []byte {
	return appendU32(buffer, uint32(id))
}

func readSize(buffer []byte) (Size, []byte, error) {
	id, buffer, err := readU32(buffer)
	return Size(id), buffer, err
}

func appendFileType(buffer []byte, id FileType) []byte {
	return appendU32(buffer, uint32(id))
}

func readFileType(buffer []byte) (FileType, []byte, error) {
	id, buffer, err := readU32(buffer)
	return FileType(id), buffer, err
}

func appendFDFlags(buffer []byte, id FDFlags) []byte {
	return appendU32(buffer, uint32(id))
}

func readFDFlags(buffer []byte) (FDFlags, []byte, error) {
	id, buffer, err := readU32(buffer)
	return FDFlags(id), buffer, err
}

func appendFSTFlags(buffer []byte, id FSTFlags) []byte {
	return appendU32(buffer, uint32(id))
}

func readFSTFlags(buffer []byte) (FSTFlags, []byte, error) {
	id, buffer, err := readU32(buffer)
	return FSTFlags(id), buffer, err
}

func appendRights(buffer []byte, id Rights) []byte {
	return appendU64(buffer, uint64(id))
}

func readRights(buffer []byte) (Rights, []byte, error) {
	id, buffer, err := readU64(buffer)
	return Rights(id), buffer, err
}

func appendFileSize(buffer []byte, id FileSize) []byte {
	return appendU64(buffer, uint64(id))
}

func readFileSize(buffer []byte) (FileSize, []byte, error) {
	id, buffer, err := readU64(buffer)
	return FileSize(id), buffer, err
}

func appendDevice(buffer []byte, id Device) []byte {
	return appendU64(buffer, uint64(id))
}

func readDevice(buffer []byte) (Device, []byte, error) {
	id, buffer, err := readU64(buffer)
	return Device(id), buffer, err
}

func appendINode(buffer []byte, id INode) []byte {
	return appendU64(buffer, uint64(id))
}

func readINode(buffer []byte) (INode, []byte, error) {
	id, buffer, err := readU64(buffer)
	return INode(id), buffer, err
}

func appendLinkCount(buffer []byte, id LinkCount) []byte {
	return appendU64(buffer, uint64(id))
}

func readLinkCount(buffer []byte) (LinkCount, []byte, error) {
	id, buffer, err := readU64(buffer)
	return LinkCount(id), buffer, err
}

func appendAdvice(buffer []byte, id Advice) []byte {
	return appendU32(buffer, uint32(id))
}

func readAdvice(buffer []byte) (Advice, []byte, error) {
	id, buffer, err := readU32(buffer)
	return Advice(id), buffer, err
}

func appendExitCode(buffer []byte, id ExitCode) []byte {
	return appendU32(buffer, uint32(id))
}

func readExitCode(buffer []byte) (ExitCode, []byte, error) {
	id, buffer, err := readU32(buffer)
	return ExitCode(id), buffer, err
}

func appendSignal(buffer []byte, id Signal) []byte {
	return appendU32(buffer, uint32(id))
}

func readSignal(buffer []byte) (Signal, []byte, error) {
	id, buffer, err := readU32(buffer)
	return Signal(id), buffer, err
}

func appendPreOpenType(buffer []byte, t PreOpenType) []byte {
	return appendU32(buffer, uint32(t))
}

func readPreOpenType(buffer []byte) (PreOpenType, []byte, error) {
	t, buffer, err := readU32(buffer)
	return PreOpenType(t), buffer, err
}

func appendUserData(buffer []byte, id UserData) []byte {
	return appendU64(buffer, uint64(id))
}

func readUserData(buffer []byte) (UserData, []byte, error) {
	id, buffer, err := readU64(buffer)
	return UserData(id), buffer, err
}

func appendEventType(buffer []byte, t EventType) []byte {
	return appendU32(buffer, uint32(t))
}

func readEventType(buffer []byte) (EventType, []byte, error) {
	t, buffer, err := readU32(buffer)
	return EventType(t), buffer, err
}

func appendSubscriptionClockFlags(buffer []byte, t SubscriptionClockFlags) []byte {
	return appendU32(buffer, uint32(t))
}

func readSubscriptionClockFlags(buffer []byte) (SubscriptionClockFlags, []byte, error) {
	t, buffer, err := readU32(buffer)
	return SubscriptionClockFlags(t), buffer, err
}

func appendEventFDReadWriteFlags(buffer []byte, t EventFDReadWriteFlags) []byte {
	return appendU32(buffer, uint32(t))
}

func readEventFDReadWriteFlags(buffer []byte) (EventFDReadWriteFlags, []byte, error) {
	t, buffer, err := readU32(buffer)
	return EventFDReadWriteFlags(t), buffer, err
}

func appendPreStat(buffer []byte, stat PreStat) []byte {
	buffer = appendPreOpenType(buffer, stat.Type)
	buffer = appendSize(buffer, stat.PreStatDir.NameLength)
	return buffer
}

func readPreStat(buffer []byte) (stat PreStat, _ []byte, err error) {
	if stat.Type, buffer, err = readPreOpenType(buffer); err != nil {
		return
	}
	if stat.PreStatDir.NameLength, buffer, err = readSize(buffer); err != nil {
		return
	}
	return
}

func appendFDStat(buffer []byte, stat FDStat) []byte {
	buffer = appendFileType(buffer, stat.FileType)
	buffer = appendFDFlags(buffer, stat.Flags)
	buffer = appendRights(buffer, stat.RightsBase)
	buffer = appendRights(buffer, stat.RightsInheriting)
	return buffer
}

func readFDStat(buffer []byte) (stat FDStat, _ []byte, err error) {
	if stat.FileType, buffer, err = readFileType(buffer); err != nil {
		return
	}
	if stat.Flags, buffer, err = readFDFlags(buffer); err != nil {
		return
	}
	if stat.RightsBase, buffer, err = readRights(buffer); err != nil {
		return
	}
	if stat.RightsInheriting, buffer, err = readRights(buffer); err != nil {
		return
	}
	return
}

func appendFileStat(buffer []byte, stat FileStat) []byte {
	buffer = appendDevice(buffer, stat.Device)
	buffer = appendINode(buffer, stat.INode)
	buffer = appendFileType(buffer, stat.FileType)
	buffer = appendLinkCount(buffer, stat.NLink)
	buffer = appendFileSize(buffer, stat.Size)
	buffer = appendTimestamp(buffer, stat.AccessTime)
	buffer = appendTimestamp(buffer, stat.ModifyTime)
	return appendTimestamp(buffer, stat.ChangeTime)
}

func readFileStat(buffer []byte) (stat FileStat, _ []byte, err error) {
	if stat.Device, buffer, err = readDevice(buffer); err != nil {
		return
	}
	if stat.INode, buffer, err = readINode(buffer); err != nil {
		return
	}
	if stat.FileType, buffer, err = readFileType(buffer); err != nil {
		return
	}
	if stat.NLink, buffer, err = readLinkCount(buffer); err != nil {
		return
	}
	if stat.Size, buffer, err = readFileSize(buffer); err != nil {
		return
	}
	if stat.AccessTime, buffer, err = readTimestamp(buffer); err != nil {
		return
	}
	if stat.ModifyTime, buffer, err = readTimestamp(buffer); err != nil {
		return
	}
	stat.ChangeTime, buffer, err = readTimestamp(buffer)
	return
}

func appendSubscription(buffer []byte, s Subscription) []byte {
	buffer = appendUserData(buffer, s.UserData)
	buffer = appendEventType(buffer, s.EventType)
	switch s.EventType {
	case FDReadEvent, FDWriteEvent:
		f := s.GetFDReadWrite()
		return appendFD(buffer, f.FD)
	case ClockEvent:
		c := s.GetClock()
		buffer = appendClockID(buffer, c.ID)
		buffer = appendTimestamp(buffer, c.Precision)
		buffer = appendTimestamp(buffer, c.Timeout)
		return appendSubscriptionClockFlags(buffer, c.Flags)
	default:
		panic("invalid subscription event type")
	}
}

func readSubscription(buffer []byte) (s Subscription, _ []byte, err error) {
	if s.UserData, buffer, err = readUserData(buffer); err != nil {
		return
	}
	if s.EventType, buffer, err = readEventType(buffer); err != nil {
		return
	}
	switch s.EventType {
	case FDReadEvent, FDWriteEvent:
		var f SubscriptionFDReadWrite
		if f.FD, buffer, err = readFD(buffer); err != nil {
			return
		}
		s.SetFDReadWrite(f)
	case ClockEvent:
		var c SubscriptionClock
		if c.ID, buffer, err = readClockID(buffer); err != nil {
			return
		}
		if c.Precision, buffer, err = readTimestamp(buffer); err != nil {
			return
		}
		if c.Timeout, buffer, err = readTimestamp(buffer); err != nil {
			return
		}
		if c.Flags, buffer, err = readSubscriptionClockFlags(buffer); err != nil {
			return
		}
		s.SetClock(c)
	default:
		err = fmt.Errorf("invalid subscription event type: %v", s.EventType)
	}
	return
}

func appendEvent(buffer []byte, e Event) []byte {
	buffer = appendUserData(buffer, e.UserData)
	buffer = appendErrno(buffer, e.Errno)
	buffer = appendEventType(buffer, e.EventType)
	switch e.EventType {
	case FDReadEvent, FDWriteEvent:
		buffer = appendFileSize(buffer, e.FDReadWrite.NBytes)
		return appendEventFDReadWriteFlags(buffer, e.FDReadWrite.Flags)
	case ClockEvent:
		return buffer
	default:
		panic("invalid event type")
	}
}

func readEvent(buffer []byte) (e Event, _ []byte, err error) {
	if e.UserData, buffer, err = readUserData(buffer); err != nil {
		return
	}
	if e.Errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if e.EventType, buffer, err = readEventType(buffer); err != nil {
		return
	}
	switch e.EventType {
	case FDReadEvent, FDWriteEvent:
		if e.FDReadWrite.NBytes, buffer, err = readFileSize(buffer); err != nil {
			return
		}
		e.FDReadWrite.Flags, buffer, err = readEventFDReadWriteFlags(buffer)
	case ClockEvent:
	default:
		err = fmt.Errorf("invalid subscription event type: %v", e.EventType)
	}
	return
}

func appendSubscriptions(buffer []byte, subscriptions []Subscription) []byte {
	buffer = appendU32(buffer, uint32(len(subscriptions)))
	for i := range subscriptions {
		buffer = appendSubscription(buffer, subscriptions[i])
	}
	return buffer
}

func readSubscriptions(buffer []byte, subscriptions []Subscription) (_ []Subscription, _ []byte, err error) {
	var count uint32
	if count, buffer, err = readU32(buffer); err != nil {
		return
	}
	if uint32(len(subscriptions)) < count {
		subscriptions = make([]Subscription, count)
	} else {
		subscriptions = subscriptions[:count]
	}
	for i := uint32(0); i < count; i++ {
		subscriptions[i], buffer, err = readSubscription(buffer)
		if err != nil {
			return
		}
	}
	return subscriptions, buffer, nil
}

func appendEvents(buffer []byte, events []Event) []byte {
	buffer = appendU32(buffer, uint32(len(events)))
	for i := range events {
		buffer = appendEvent(buffer, events[i])
	}
	return buffer
}

func readEvents(buffer []byte, events []Event) (_ []Event, _ []byte, err error) {
	var count uint32
	if count, buffer, err = readU32(buffer); err != nil {
		return
	}
	if uint32(len(events)) < count {
		events = make([]Event, count)
	} else {
		events = events[:count]
	}
	for i := uint32(0); i < count; i++ {
		events[i], buffer, err = readEvent(buffer)
		if err != nil {
			return
		}
	}
	return events, buffer, nil
}
