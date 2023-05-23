package wasicall

import (
	"encoding/binary"
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
	buffer = appendStrings(buffer, args)
	return buffer
}

func (c *Codec) DecodeArgsGet(buffer []byte, args []string) (_ []string, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if args, buffer, err = readStrings(buffer, args); err != nil {
		return
	}
	return
}

func (c *Codec) EncodeEnvironGet(buffer []byte, env []string, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendStrings(buffer, env)
	return buffer
}

func (c *Codec) DecodeEnvironGet(buffer []byte, env []string) (_ []string, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if env, buffer, err = readStrings(buffer, env); err != nil {
		return
	}
	return
}

func (c *Codec) EncodeClockResGet(buffer []byte, id ClockID, timestamp Timestamp, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendClockID(buffer, id)
	buffer = appendTimestamp(buffer, timestamp)
	return buffer
}

func (c *Codec) DecodeClockResGet(buffer []byte) (id ClockID, timestamp Timestamp, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if id, buffer, err = readClockID(buffer); err != nil {
		return
	}
	if timestamp, buffer, err = readTimestamp(buffer); err != nil {
		return
	}
	return
}

func (c *Codec) EncodeClockTimeGet(buffer []byte, id ClockID, precision Timestamp, timestamp Timestamp, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendClockID(buffer, id)
	buffer = appendTimestamp(buffer, precision)
	buffer = appendTimestamp(buffer, timestamp)
	return buffer
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
	if timestamp, buffer, err = readTimestamp(buffer); err != nil {
		return
	}
	return
}

func (c *Codec) EncodeFDAdvise(buffer []byte, fd FD, offset FileSize, length FileSize, advice Advice, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDAdvise(buffer []byte) (fd FD, offset FileSize, length FileSize, advice Advice, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDAllocate(buffer []byte, fd FD, offset FileSize, length FileSize, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDAllocate(buffer []byte) (fd FD, offset FileSize, length FileSize, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDClose(buffer []byte, fd FD, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDClose(buffer []byte) (fd FD, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDDataSync(buffer []byte, fd FD, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDDataSync(buffer []byte) (fd FD, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDStatGet(buffer []byte, fd FD, stat FDStat, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendFD(buffer, fd)
	buffer = appendFDStat(buffer, stat)
	return buffer
}

func (c *Codec) DecodeFDStatGet(buffer []byte) (fd FD, stat FDStat, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if stat, buffer, err = readFDStat(buffer); err != nil {
		return
	}
	return
}

func (c *Codec) EncodeFDStatSetFlags(buffer []byte, fd FD, flags FDFlags, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDStatSetFlags(buffer []byte) (fd FD, flags FDFlags, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDStatSetRights(buffer []byte, fd FD, rightsBase, rightsInheriting Rights, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDStatSetRights(buffer []byte) (fd FD, rightsBase, rightsInheriting Rights, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDFileStatGet(buffer []byte, fd FD, stat FileStat, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDFileStatGet(buffer []byte) (fd FD, stat FileStat, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDFileStatSetSize(buffer []byte, fd FD, size FileSize, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDFileStatSetSize(buffer []byte) (fd FD, size FileSize, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeFDFileStatSetTimes(buffer []byte, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeFDFileStatSetTimes(buffer []byte) (fd FD, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno, err error) {
	panic("not implemented")
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
	buffer = appendPreStat(buffer, stat)
	return buffer
}

func (c *Codec) DecodeFDPreStatGet(buffer []byte) (fd FD, stat PreStat, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = readFD(buffer); err != nil {
		return
	}
	if stat, buffer, err = readPreStat(buffer); err != nil {
		return
	}
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
	panic("not implemented")
}

func (c *Codec) DecodeFDRead(buffer []byte) (fd FD, iovecs []IOVec, size Size, errno Errno, err error) {
	panic("not implemented")
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
	panic("not implemented")
}

func (c *Codec) DecodeFDSync(buffer []byte) (fd FD, errno Errno, err error) {
	panic("not implemented")
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
	buffer = appendSize(buffer, size)
	return buffer
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
	if size, buffer, err = readSize(buffer); err != nil {
		return
	}
	return
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
	panic("not implemented")
}

func (c *Codec) DecodePollOneOff(buffer []byte) (subscriptions []Subscription, events []Event, n int, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeProcExit(buffer []byte, exitCode ExitCode, errno Errno) []byte {
	buffer = appendErrno(buffer, errno)
	buffer = appendExitCode(buffer, exitCode)
	return buffer
}

func (c *Codec) DecodeProcExit(buffer []byte) (exitCode ExitCode, errno Errno, err error) {
	if errno, buffer, err = readErrno(buffer); err != nil {
		return
	}
	if exitCode, buffer, err = readExitCode(buffer); err != nil {
		return
	}
	return
}

func (c *Codec) EncodeProcRaise(buffer []byte, signal Signal, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeProcRaise(buffer []byte) (signal Signal, errno Errno, err error) {
	panic("not implemented")
}

func (c *Codec) EncodeSchedYield(buffer []byte, errno Errno) []byte {
	panic("not implemented")
}

func (c *Codec) DecodeSchedYield(buffer []byte) (errno Errno, err error) {
	panic("not implemented")
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
	panic("not implemented")
}

func (c *Codec) DecodeSockAccept(buffer []byte) (fd FD, flags FDFlags, newfd FD, errno Errno, err error) {
	panic("not implemented")
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
	return buffer[:length], buffer, err
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

func appendRights(buffer []byte, id Rights) []byte {
	return appendU64(buffer, uint64(id))
}

func readRights(buffer []byte) (Rights, []byte, error) {
	id, buffer, err := readU64(buffer)
	return Rights(id), buffer, err
}

func appendExitCode(buffer []byte, id ExitCode) []byte {
	return appendU32(buffer, uint32(id))
}

func readExitCode(buffer []byte) (ExitCode, []byte, error) {
	id, buffer, err := readU32(buffer)
	return ExitCode(id), buffer, err
}

func appendPreOpenType(buffer []byte, t PreOpenType) []byte {
	return appendU32(buffer, uint32(t))
}

func readPreOpenType(buffer []byte) (PreOpenType, []byte, error) {
	t, buffer, err := readU32(buffer)
	return PreOpenType(t), buffer, err
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
