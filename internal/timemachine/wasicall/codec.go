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
	buffer = appendU32(buffer, uint32(errno))
	buffer = appendU32(buffer, uint32(len(args)))
	for _, arg := range args {
		buffer = appendU32(buffer, uint32(len(arg)))
		buffer = append(buffer, arg...)
	}
	return buffer
}

func (c *Codec) DecodeArgsGet(buffer []byte, args []string) (a []string, errno Errno, err error) {
	var errnoU32 uint32
	if errnoU32, buffer, err = readU32(buffer); err != nil {
		return
	}
	var argc uint32
	if argc, buffer, err = readU32(buffer); err != nil {
		return
	}
	for i := uint32(0); i < argc; i++ {
		var length uint32
		if argc, buffer, err = readU32(buffer); err != nil {
			return
		}
		if uint32(len(buffer)) < length {
			err = io.ErrShortBuffer
			return
		}
		args = append(args, unsafe.String(&buffer[0], length))
		buffer = buffer[length:]
	}
	return args, Errno(errnoU32), nil
}

func (c *Codec) EncodeEnvironGet(buffer []byte, env []string, errno Errno) []byte {
	return c.EncodeArgsGet(buffer, env, errno)
}

func (c *Codec) DecodeEnvironGet(buffer []byte, env []string) ([]string, Errno, error) {
	return c.DecodeArgsGet(buffer, env)
}

func (c *Codec) EncodeClockResGet(buffer []byte, id ClockID, timestamp Timestamp, errno Errno) []byte {
	buffer = appendU32(buffer, uint32(errno))
	buffer = appendU32(buffer, uint32(id))
	buffer = appendU64(buffer, uint64(timestamp))
	return buffer
}

func (c *Codec) DecodeClockResGet(buffer []byte) (id ClockID, timestamp Timestamp, errno Errno, err error) {
	var errnoU32 uint32
	if errnoU32, buffer, err = readU32(buffer); err != nil {
		return
	}
	var idU32 uint32
	if idU32, buffer, err = readU32(buffer); err != nil {
		return
	}
	var timestampU64 uint64
	if timestampU64, buffer, err = readU64(buffer); err != nil {
		return
	}
	return ClockID(idU32), Timestamp(timestampU64), Errno(errnoU32), nil
}

func (c *Codec) EncodeClockTimeGet(buffer []byte, id ClockID, precision Timestamp, timestamp Timestamp, errno Errno) []byte {
	buffer = appendU32(buffer, uint32(errno))
	buffer = appendU32(buffer, uint32(id))
	buffer = appendU64(buffer, uint64(precision))
	buffer = appendU64(buffer, uint64(timestamp))
	return buffer
}

func (c *Codec) DecodeClockTimeGet(buffer []byte) (id ClockID, precision Timestamp, timestamp Timestamp, errno Errno, err error) {
	var errnoU32 uint32
	if errnoU32, buffer, err = readU32(buffer); err != nil {
		return
	}
	var idU32 uint32
	if idU32, buffer, err = readU32(buffer); err != nil {
		return
	}
	var precisionU64 uint64
	if precisionU64, buffer, err = readU64(buffer); err != nil {
		return
	}
	var timestampU64 uint64
	if timestampU64, buffer, err = readU64(buffer); err != nil {
		return
	}
	return ClockID(idU32), Timestamp(precisionU64), Timestamp(timestampU64), Errno(errnoU32), nil
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
	panic("not implemented")
}

func (c *Codec) DecodeFDStatGet(buffer []byte) (fd FD, stat FDStat, errno Errno, err error) {
	panic("not implemented")
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
	panic("not implemented")
}

func (c *Codec) DecodeFDPreStatGet(buffer []byte) (fd FD, stat PreStat, errno Errno, err error) {
	panic("not implemented")
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
	panic("not implemented")
}

func (c *Codec) DecodeFDWrite(buffer []byte) (fd FD, iovecs []IOVec, size Size, errno Errno, err error) {
	panic("not implemented")
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
	panic("not implemented")
}

func (c *Codec) DecodeProcExit(buffer []byte) (exitCode ExitCode, errno Errno, err error) {
	panic("not implemented")
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
	buffer = appendU32(buffer, uint32(errno))
	buffer = appendU32(buffer, uint32(len(b)))
	return append(buffer, b...)
}

func (c *Codec) DecodeRandomGet(buffer []byte) (buf []byte, errno Errno, err error) {
	var errnoU32 uint32
	if errnoU32, buffer, err = readU32(buffer); err != nil {
		return
	}
	var length uint32
	if length, buffer, err = readU32(buffer); err != nil {
		return
	}
	if uint32(len(buffer)) < length {
		err = io.ErrShortBuffer
		return
	}
	buf = buffer[:length]
	errno = Errno(errnoU32)
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

func appendU64(b []byte, v uint64) []byte {
	return binary.LittleEndian.AppendUint64(b, v)
}

func readU32(b []byte) (uint32, []byte, error) {
	if len(b) < 4 {
		return 0, nil, io.ErrShortBuffer
	}
	return binary.LittleEndian.Uint32(b), b[4:], nil
}

func readU64(b []byte) (uint64, []byte, error) {
	if len(b) < 8 {
		return 0, nil, io.ErrShortBuffer
	}
	return binary.LittleEndian.Uint64(b), b[8:], nil
}
