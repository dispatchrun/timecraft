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
// the records are ultimately compressed.
//
// Other things that have been tried:
//   - varints (bb159c7, discussed in stealthrocket/timecraft#11)
//   - omitting return values except for errno when errno!=ESUCCESS (532bfbb,
//     discussed in stealthrocket/timecraft#11)
type Codec struct{}

func (c *Codec) EncodeArgsSizesGet(buffer []byte, argCount, stringBytes int, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeInt(buffer, argCount)
	return encodeInt(buffer, stringBytes)
}

func (c *Codec) DecodeArgsSizesGet(buffer []byte) (argCount, stringBytes int, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if argCount, buffer, err = decodeInt(buffer); err != nil {
		return
	}
	stringBytes, buffer, err = decodeInt(buffer)
	return
}

func (c *Codec) EncodeArgsGet(buffer []byte, args []string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	return encodeStrings(buffer, args)
}

func (c *Codec) DecodeArgsGet(buffer []byte, args []string) (_ []string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	args, buffer, err = decodeStrings(buffer, args)
	return args, errno, err
}

func (c *Codec) EncodeEnvironSizesGet(buffer []byte, envCount, stringBytes int, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeInt(buffer, envCount)
	return encodeInt(buffer, stringBytes)
}

func (c *Codec) DecodeEnvironSizesGet(buffer []byte) (envCount, stringBytes int, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if envCount, buffer, err = decodeInt(buffer); err != nil {
		return
	}
	stringBytes, buffer, err = decodeInt(buffer)
	return
}

func (c *Codec) EncodeEnvironGet(buffer []byte, env []string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	return encodeStrings(buffer, env)
}

func (c *Codec) DecodeEnvironGet(buffer []byte, env []string) (_ []string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	env, buffer, err = decodeStrings(buffer, env)
	return env, errno, err
}

func (c *Codec) EncodeClockResGet(buffer []byte, id ClockID, timestamp Timestamp, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeClockID(buffer, id)
	return encodeTimestamp(buffer, timestamp)
}

func (c *Codec) DecodeClockResGet(buffer []byte) (id ClockID, timestamp Timestamp, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if id, buffer, err = decodeClockID(buffer); err != nil {
		return
	}
	timestamp, buffer, err = decodeTimestamp(buffer)
	return
}

func (c *Codec) EncodeClockTimeGet(buffer []byte, id ClockID, precision Timestamp, timestamp Timestamp, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeClockID(buffer, id)
	buffer = encodeTimestamp(buffer, precision)
	return encodeTimestamp(buffer, timestamp)
}

func (c *Codec) DecodeClockTimeGet(buffer []byte) (id ClockID, precision Timestamp, timestamp Timestamp, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if id, buffer, err = decodeClockID(buffer); err != nil {
		return
	}
	if precision, buffer, err = decodeTimestamp(buffer); err != nil {
		return
	}
	timestamp, buffer, err = decodeTimestamp(buffer)
	return
}

func (c *Codec) EncodeFDAdvise(buffer []byte, fd FD, offset FileSize, length FileSize, advice Advice, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeFileSize(buffer, offset)
	buffer = encodeFileSize(buffer, length)
	return encodeAdvice(buffer, advice)
}

func (c *Codec) DecodeFDAdvise(buffer []byte) (fd FD, offset FileSize, length FileSize, advice Advice, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if offset, buffer, err = decodeFileSize(buffer); err != nil {
		return
	}
	if length, buffer, err = decodeFileSize(buffer); err != nil {
		return
	}
	advice, buffer, err = decodeAdvice(buffer)
	return
}

func (c *Codec) EncodeFDAllocate(buffer []byte, fd FD, offset FileSize, length FileSize, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeFileSize(buffer, offset)
	return encodeFileSize(buffer, length)
}

func (c *Codec) DecodeFDAllocate(buffer []byte) (fd FD, offset FileSize, length FileSize, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if offset, buffer, err = decodeFileSize(buffer); err != nil {
		return
	}
	length, buffer, err = decodeFileSize(buffer)
	return
}

func (c *Codec) EncodeFDClose(buffer []byte, fd FD, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	return encodeFD(buffer, fd)
}

func (c *Codec) DecodeFDClose(buffer []byte) (fd FD, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	fd, buffer, err = decodeFD(buffer)
	return
}

func (c *Codec) EncodeFDDataSync(buffer []byte, fd FD, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	return encodeFD(buffer, fd)
}

func (c *Codec) DecodeFDDataSync(buffer []byte) (fd FD, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	fd, buffer, err = decodeFD(buffer)
	return
}

func (c *Codec) EncodeFDStatGet(buffer []byte, fd FD, stat FDStat, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeFDStat(buffer, stat)
}

func (c *Codec) DecodeFDStatGet(buffer []byte) (fd FD, stat FDStat, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	stat, buffer, err = decodeFDStat(buffer)
	return
}

func (c *Codec) EncodeFDStatSetFlags(buffer []byte, fd FD, flags FDFlags, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeFDFlags(buffer, flags)
}

func (c *Codec) DecodeFDStatSetFlags(buffer []byte) (fd FD, flags FDFlags, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	flags, buffer, err = decodeFDFlags(buffer)
	return
}

func (c *Codec) EncodeFDStatSetRights(buffer []byte, fd FD, rightsBase, rightsInheriting Rights, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeRights(buffer, rightsBase)
	return encodeRights(buffer, rightsInheriting)
}

func (c *Codec) DecodeFDStatSetRights(buffer []byte) (fd FD, rightsBase, rightsInheriting Rights, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if rightsBase, buffer, err = decodeRights(buffer); err != nil {
		return
	}
	rightsInheriting, buffer, err = decodeRights(buffer)
	return
}

func (c *Codec) EncodeFDFileStatGet(buffer []byte, fd FD, stat FileStat, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeFileStat(buffer, stat)
}

func (c *Codec) DecodeFDFileStatGet(buffer []byte) (fd FD, stat FileStat, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	stat, buffer, err = decodeFileStat(buffer)
	return
}

func (c *Codec) EncodeFDFileStatSetSize(buffer []byte, fd FD, size FileSize, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeFileSize(buffer, size)
}

func (c *Codec) DecodeFDFileStatSetSize(buffer []byte) (fd FD, size FileSize, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	size, buffer, err = decodeFileSize(buffer)
	return
}

func (c *Codec) EncodeFDFileStatSetTimes(buffer []byte, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeTimestamp(buffer, accessTime)
	buffer = encodeTimestamp(buffer, modifyTime)
	return encodeFSTFlags(buffer, flags)
}

func (c *Codec) DecodeFDFileStatSetTimes(buffer []byte) (fd FD, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if accessTime, buffer, err = decodeTimestamp(buffer); err != nil {
		return
	}
	if modifyTime, buffer, err = decodeTimestamp(buffer); err != nil {
		return
	}
	flags, buffer, err = decodeFSTFlags(buffer)
	return
}

func (c *Codec) EncodeFDPread(buffer []byte, fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeIOVecsPrefix(buffer, iovecs, size)
	buffer = encodeFileSize(buffer, offset)
	return encodeSize(buffer, size)
}

func (c *Codec) DecodeFDPread(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, offset FileSize, size Size, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = decodeIOVecs(buffer, iovecs); err != nil {
		return
	}
	if offset, buffer, err = decodeFileSize(buffer); err != nil {
		return
	}
	size, buffer, err = decodeSize(buffer)
	return fd, iovecs, offset, size, errno, err
}

func (c *Codec) EncodeFDPreStatGet(buffer []byte, fd FD, stat PreStat, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodePreStat(buffer, stat)
}

func (c *Codec) DecodeFDPreStatGet(buffer []byte) (fd FD, stat PreStat, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	stat, buffer, err = decodePreStat(buffer)
	return
}

func (c *Codec) EncodeFDPreStatDirName(buffer []byte, fd FD, name string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeString(buffer, name)
}

func (c *Codec) DecodeFDPreStatDirName(buffer []byte) (fd FD, name string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	name, buffer, err = decodeString(buffer)
	return
}

func (c *Codec) EncodeFDPwrite(buffer []byte, fd FD, iovecs []IOVec, offset FileSize, size Size, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeIOVecs(buffer, iovecs)
	buffer = encodeFileSize(buffer, offset)
	return encodeSize(buffer, size)
}

func (c *Codec) DecodeFDPwrite(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, offset FileSize, size Size, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = decodeIOVecs(buffer, iovecs); err != nil {
		return
	}
	if offset, buffer, err = decodeFileSize(buffer); err != nil {
		return
	}
	size, buffer, err = decodeSize(buffer)
	return fd, iovecs, offset, size, errno, err
}

func (c *Codec) EncodeFDRead(buffer []byte, fd FD, iovecs []IOVec, size Size, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeIOVecsPrefix(buffer, iovecs, size)
	return encodeSize(buffer, size)
}

func (c *Codec) DecodeFDRead(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, size Size, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = decodeIOVecs(buffer, iovecs); err != nil {
		return
	}
	size, buffer, err = decodeSize(buffer)
	return fd, iovecs, size, errno, err
}

func (c *Codec) EncodeFDReadDir(buffer []byte, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int, count int, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeDirEntries(buffer, entries)
	buffer = encodeDirCookie(buffer, cookie)
	buffer = encodeInt(buffer, bufferSizeBytes)
	return encodeInt(buffer, count)
}

func (c *Codec) DecodeFDReadDir(buffer []byte, entries []DirEntry) (fd FD, _ []DirEntry, cookie DirCookie, bufferSizeBytes int, count int, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if entries, buffer, err = decodeDirEntries(buffer, entries); err != nil {
		return
	}
	if cookie, buffer, err = decodeDirCookie(buffer); err != nil {
		return
	}
	if bufferSizeBytes, buffer, err = decodeInt(buffer); err != nil {
		return
	}
	count, buffer, err = decodeInt(buffer)
	return fd, entries, cookie, bufferSizeBytes, count, errno, err
}

func (c *Codec) EncodeFDRenumber(buffer []byte, from, to FD, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, from)
	return encodeFD(buffer, to)
}

func (c *Codec) DecodeFDRenumber(buffer []byte) (from, to FD, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if from, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	to, buffer, err = decodeFD(buffer)
	return
}

func (c *Codec) EncodeFDSeek(buffer []byte, fd FD, seekOffset FileDelta, whence Whence, offset FileSize, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeFileDelta(buffer, seekOffset)
	buffer = encodeWhence(buffer, whence)
	return encodeFileSize(buffer, offset)
}

func (c *Codec) DecodeFDSeek(buffer []byte) (fd FD, seekOffset FileDelta, whence Whence, offset FileSize, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if seekOffset, buffer, err = decodeFileDelta(buffer); err != nil {
		return
	}
	if whence, buffer, err = decodeWhence(buffer); err != nil {
		return
	}
	offset, buffer, err = decodeFileSize(buffer)
	return
}

func (c *Codec) EncodeFDSync(buffer []byte, fd FD, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	return encodeFD(buffer, fd)
}

func (c *Codec) DecodeFDSync(buffer []byte) (fd FD, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	fd, buffer, err = decodeFD(buffer)
	return
}

func (c *Codec) EncodeFDTell(buffer []byte, fd FD, offset FileSize, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeFileSize(buffer, offset)
}

func (c *Codec) DecodeFDTell(buffer []byte) (fd FD, offset FileSize, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	offset, buffer, err = decodeFileSize(buffer)
	return
}

func (c *Codec) EncodeFDWrite(buffer []byte, fd FD, iovecs []IOVec, size Size, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeIOVecs(buffer, iovecs)
	return encodeSize(buffer, size)
}

func (c *Codec) DecodeFDWrite(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, size Size, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = decodeIOVecs(buffer, iovecs); err != nil {
		return
	}
	size, buffer, err = decodeSize(buffer)
	return fd, iovecs, size, errno, err
}

func (c *Codec) EncodePathCreateDirectory(buffer []byte, fd FD, path string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeString(buffer, path)
}

func (c *Codec) DecodePathCreateDirectory(buffer []byte) (fd FD, path string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	path, buffer, err = decodeString(buffer)
	return
}

func (c *Codec) EncodePathFileStatGet(buffer []byte, fd FD, lookupFlags LookupFlags, path string, fileStat FileStat, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeLookupFlags(buffer, lookupFlags)
	buffer = encodeString(buffer, path)
	return encodeFileStat(buffer, fileStat)
}

func (c *Codec) DecodePathFileStatGet(buffer []byte) (fd FD, lookupFlags LookupFlags, path string, fileStat FileStat, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if lookupFlags, buffer, err = decodeLookupFlags(buffer); err != nil {
		return
	}
	if path, buffer, err = decodeString(buffer); err != nil {
		return
	}
	fileStat, buffer, err = decodeFileStat(buffer)
	return
}

func (c *Codec) EncodePathFileStatSetTimes(buffer []byte, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeLookupFlags(buffer, lookupFlags)
	buffer = encodeString(buffer, path)
	buffer = encodeTimestamp(buffer, accessTime)
	buffer = encodeTimestamp(buffer, modifyTime)
	return encodeFSTFlags(buffer, flags)
}

func (c *Codec) DecodePathFileStatSetTimes(buffer []byte) (fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if lookupFlags, buffer, err = decodeLookupFlags(buffer); err != nil {
		return
	}
	if path, buffer, err = decodeString(buffer); err != nil {
		return
	}
	if accessTime, buffer, err = decodeTimestamp(buffer); err != nil {
		return
	}
	if modifyTime, buffer, err = decodeTimestamp(buffer); err != nil {
		return
	}
	flags, buffer, err = decodeFSTFlags(buffer)
	return
}

func (c *Codec) EncodePathLink(buffer []byte, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, oldFD)
	buffer = encodeLookupFlags(buffer, oldFlags)
	buffer = encodeString(buffer, oldPath)
	buffer = encodeFD(buffer, newFD)
	return encodeString(buffer, newPath)
}

func (c *Codec) DecodePathLink(buffer []byte) (oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if oldFD, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if oldFlags, buffer, err = decodeLookupFlags(buffer); err != nil {
		return
	}
	if oldPath, buffer, err = decodeString(buffer); err != nil {
		return
	}
	if newFD, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	newPath, buffer, err = decodeString(buffer)
	return
}

func (c *Codec) EncodePathOpen(buffer []byte, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags, newfd FD, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeLookupFlags(buffer, dirFlags)
	buffer = encodeString(buffer, path)
	buffer = encodeOpenFlags(buffer, openFlags)
	buffer = encodeRights(buffer, rightsBase)
	buffer = encodeRights(buffer, rightsInheriting)
	buffer = encodeFDFlags(buffer, fdFlags)
	return encodeFD(buffer, newfd)
}

func (c *Codec) DecodePathOpen(buffer []byte) (fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags, newfd FD, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if dirFlags, buffer, err = decodeLookupFlags(buffer); err != nil {
		return
	}
	if path, buffer, err = decodeString(buffer); err != nil {
		return
	}
	if openFlags, buffer, err = decodeOpenFlags(buffer); err != nil {
		return
	}
	if rightsBase, buffer, err = decodeRights(buffer); err != nil {
		return
	}
	if rightsInheriting, buffer, err = decodeRights(buffer); err != nil {
		return
	}
	fdFlags, buffer, err = decodeFDFlags(buffer)
	return
}

func (c *Codec) EncodePathReadLink(buffer []byte, fd FD, path string, output []byte, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeString(buffer, path)
	return encodeBytes(buffer, output)
}

func (c *Codec) DecodePathReadLink(buffer []byte) (fd FD, path string, output []byte, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if path, buffer, err = decodeString(buffer); err != nil {
		return
	}
	output, buffer, err = decodeBytes(buffer)
	return
}

func (c *Codec) EncodePathRemoveDirectory(buffer []byte, fd FD, path string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeString(buffer, path)
}

func (c *Codec) DecodePathRemoveDirectory(buffer []byte) (fd FD, path string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	path, buffer, err = decodeString(buffer)
	return
}

func (c *Codec) EncodePathRename(buffer []byte, fd FD, oldPath string, newFD FD, newPath string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeString(buffer, oldPath)
	buffer = encodeFD(buffer, newFD)
	return encodeString(buffer, newPath)
}

func (c *Codec) DecodePathRename(buffer []byte) (fd FD, oldPath string, newFD FD, newPath string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if oldPath, buffer, err = decodeString(buffer); err != nil {
		return
	}
	if newFD, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	newPath, buffer, err = decodeString(buffer)
	return
}

func (c *Codec) EncodePathSymlink(buffer []byte, oldPath string, fd FD, newPath string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeString(buffer, oldPath)
	return encodeString(buffer, newPath)
}

func (c *Codec) DecodePathSymlink(buffer []byte) (oldPath string, fd FD, newPath string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	newPath, buffer, err = decodeString(buffer)
	return
}

func (c *Codec) EncodePathUnlinkFile(buffer []byte, fd FD, path string, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeString(buffer, path)
}

func (c *Codec) DecodePathUnlinkFile(buffer []byte) (fd FD, path string, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	path, buffer, err = decodeString(buffer)
	return
}

func (c *Codec) EncodePollOneOff(buffer []byte, subscriptions []Subscription, events []Event, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeSubscriptions(buffer, subscriptions)
	return encodeEvents(buffer, events)
}

func (c *Codec) DecodePollOneOff(buffer []byte, subscriptions []Subscription, events []Event) (_ []Subscription, _ []Event, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if subscriptions, buffer, err = decodeSubscriptions(buffer, subscriptions); err != nil {
		return
	}
	if events, buffer, err = decodeEvents(buffer, events); err != nil {
		return
	}
	return subscriptions, events, errno, err
}

func (c *Codec) EncodeProcExit(buffer []byte, exitCode ExitCode, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	return encodeExitCode(buffer, exitCode)
}

func (c *Codec) DecodeProcExit(buffer []byte) (exitCode ExitCode, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	exitCode, buffer, err = decodeExitCode(buffer)
	return
}

func (c *Codec) EncodeProcRaise(buffer []byte, signal Signal, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	return encodeSignal(buffer, signal)
}

func (c *Codec) DecodeProcRaise(buffer []byte) (signal Signal, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	signal, buffer, err = decodeSignal(buffer)
	return
}

func (c *Codec) EncodeSchedYield(buffer []byte, errno Errno) []byte {
	return encodeErrno(buffer, errno)
}

func (c *Codec) DecodeSchedYield(buffer []byte) (errno Errno, err error) {
	errno, buffer, err = decodeErrno(buffer)
	return
}

func (c *Codec) EncodeRandomGet(buffer []byte, b []byte, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeBytes(buffer, b)
	return buffer
}

func (c *Codec) DecodeRandomGet(buffer []byte) (result []byte, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	result, buffer, err = decodeBytes(buffer)
	return
}

func (c *Codec) EncodeSockAccept(buffer []byte, fd FD, flags FDFlags, newfd FD, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeFDFlags(buffer, flags)
	return encodeFD(buffer, newfd)
}

func (c *Codec) DecodeSockAccept(buffer []byte) (fd FD, flags FDFlags, newfd FD, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if flags, buffer, err = decodeFDFlags(buffer); err != nil {
		return
	}
	newfd, buffer, err = decodeFD(buffer)
	return
}

func (c *Codec) EncodeSockRecv(buffer []byte, fd FD, iovecs []IOVec, iflags RIFlags, size Size, oflags ROFlags, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeIOVecsPrefix(buffer, iovecs, size)
	buffer = encodeRIFlags(buffer, iflags)
	buffer = encodeSize(buffer, size)
	return encodeROFlags(buffer, oflags)
}

func (c *Codec) DecodeSockRecv(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, iflags RIFlags, size Size, oflags ROFlags, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = decodeIOVecs(buffer, iovecs); err != nil {
		return
	}
	if iflags, buffer, err = decodeRIFlags(buffer); err != nil {
		return
	}
	if size, buffer, err = decodeSize(buffer); err != nil {
		return
	}
	oflags, buffer, err = decodeROFlags(buffer)
	return fd, iovecs, iflags, size, oflags, errno, err
}

func (c *Codec) EncodeSockSend(buffer []byte, fd FD, iovecs []IOVec, flags SIFlags, size Size, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeIOVecs(buffer, iovecs)
	buffer = encodeSIFlags(buffer, flags)
	return encodeSize(buffer, size)
}

func (c *Codec) DecodeSockSend(buffer []byte) (fd FD, iovecs []IOVec, flags SIFlags, size Size, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = decodeIOVecs(buffer, iovecs); err != nil {
		return
	}
	if flags, buffer, err = decodeSIFlags(buffer); err != nil {
		return
	}
	size, buffer, err = decodeSize(buffer)
	return fd, iovecs, flags, size, errno, err
}

func (c *Codec) EncodeSockShutdown(buffer []byte, fd FD, flags SDFlags, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeSDFlags(buffer, flags)
}

func (c *Codec) DecodeSockShutdown(buffer []byte) (fd FD, flags SDFlags, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	flags, buffer, err = decodeSDFlags(buffer)
	return
}

func (c *Codec) EncodeSockOpen(buffer []byte, family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights, fd FD, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeProtocolFamily(buffer, family)
	buffer = encodeSocketType(buffer, socketType)
	buffer = encodeProtocol(buffer, protocol)
	buffer = encodeRights(buffer, rightsBase)
	buffer = encodeRights(buffer, rightsInheriting)
	return encodeFD(buffer, fd)
}

func (c *Codec) DecodeSockOpen(buffer []byte) (family ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights, fd FD, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if family, buffer, err = decodeProtocolFamily(buffer); err != nil {
		return
	}
	if socketType, buffer, err = decodeSocketType(buffer); err != nil {
		return
	}
	if protocol, buffer, err = decodeProtocol(buffer); err != nil {
		return
	}
	if rightsBase, buffer, err = decodeRights(buffer); err != nil {
		return
	}
	if rightsInheriting, buffer, err = decodeRights(buffer); err != nil {
		return
	}
	fd, buffer, err = decodeFD(buffer)
	return
}

func (c *Codec) EncodeSockBind(buffer []byte, fd FD, addr SocketAddress, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeAddr(buffer, addr)
}

func (c *Codec) DecodeSockBind(buffer []byte) (fd FD, addr SocketAddress, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	addr, buffer, err = decodeAddr(buffer)
	return
}

func (c *Codec) EncodeSockConnect(buffer []byte, fd FD, addr SocketAddress, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeAddr(buffer, addr)
}

func (c *Codec) DecodeSockConnect(buffer []byte) (fd FD, addr SocketAddress, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	addr, buffer, err = decodeAddr(buffer)
	return
}

func (c *Codec) EncodeSockListen(buffer []byte, fd FD, backlog int, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeInt(buffer, backlog)
}

func (c *Codec) DecodeSockListen(buffer []byte) (fd FD, backlog int, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	backlog, buffer, err = decodeInt(buffer)
	return
}

func (c *Codec) EncodeSockSendTo(buffer []byte, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress, size Size, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeIOVecs(buffer, iovecs)
	buffer = encodeSIFlags(buffer, iflags)
	buffer = encodeAddr(buffer, addr)
	return encodeSize(buffer, size)
}

func (c *Codec) DecodeSockSendTo(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, iflags SIFlags, addr SocketAddress, size Size, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = decodeIOVecs(buffer, iovecs); err != nil {
		return
	}
	if iflags, buffer, err = decodeSIFlags(buffer); err != nil {
		return
	}
	if addr, buffer, err = decodeAddr(buffer); err != nil {
		return
	}
	size, buffer, err = decodeSize(buffer)
	return fd, iovecs, iflags, addr, size, errno, err
}

func (c *Codec) EncodeSockRecvFrom(buffer []byte, fd FD, iovecs []IOVec, iflags RIFlags, size Size, oflags ROFlags, addr SocketAddress, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeIOVecsPrefix(buffer, iovecs, size)
	buffer = encodeRIFlags(buffer, iflags)
	buffer = encodeSize(buffer, size)
	buffer = encodeROFlags(buffer, oflags)
	return encodeAddr(buffer, addr)
}

func (c *Codec) DecodeSockRecvFrom(buffer []byte, iovecs []IOVec) (fd FD, _ []IOVec, iflags RIFlags, size Size, oflags ROFlags, addr SocketAddress, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if iovecs, buffer, err = decodeIOVecs(buffer, iovecs); err != nil {
		return
	}
	if iflags, buffer, err = decodeRIFlags(buffer); err != nil {
		return
	}
	if size, buffer, err = decodeSize(buffer); err != nil {
		return
	}
	if oflags, buffer, err = decodeROFlags(buffer); err != nil {
		return
	}
	addr, buffer, err = decodeAddr(buffer)
	return fd, iovecs, iflags, size, oflags, addr, errno, err
}

func (c *Codec) EncodeSockGetOptInt(buffer []byte, fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeSocketOptionLevel(buffer, level)
	buffer = encodeSocketOption(buffer, option)
	return encodeInt(buffer, value)
}

func (c *Codec) DecodeSockGetOptInt(buffer []byte) (fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if level, buffer, err = decodeSocketOptionLevel(buffer); err != nil {
		return
	}
	if option, buffer, err = decodeSocketOption(buffer); err != nil {
		return
	}
	value, buffer, err = decodeInt(buffer)
	return
}

func (c *Codec) EncodeSockSetOptInt(buffer []byte, fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	buffer = encodeSocketOptionLevel(buffer, level)
	buffer = encodeSocketOption(buffer, option)
	return encodeInt(buffer, value)
}

func (c *Codec) DecodeSockSetOptInt(buffer []byte) (fd FD, level SocketOptionLevel, option SocketOption, value int, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	if level, buffer, err = decodeSocketOptionLevel(buffer); err != nil {
		return
	}
	if option, buffer, err = decodeSocketOption(buffer); err != nil {
		return
	}
	value, buffer, err = decodeInt(buffer)
	return
}

func (c *Codec) EncodeSockLocalAddress(buffer []byte, fd FD, addr SocketAddress, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeAddr(buffer, addr)
}

func (c *Codec) DecodeSockLocalAddress(buffer []byte) (fd FD, addr SocketAddress, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	addr, buffer, err = decodeAddr(buffer)
	return
}

func (c *Codec) EncodeSockPeerAddress(buffer []byte, fd FD, addr SocketAddress, errno Errno) []byte {
	buffer = encodeErrno(buffer, errno)
	buffer = encodeFD(buffer, fd)
	return encodeAddr(buffer, addr)
}

func (c *Codec) DecodeSockPeerAddress(buffer []byte) (fd FD, addr SocketAddress, errno Errno, err error) {
	if errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if fd, buffer, err = decodeFD(buffer); err != nil {
		return
	}
	addr, buffer, err = decodeAddr(buffer)
	return
}

func encodeU32(b []byte, v uint32) []byte {
	return binary.LittleEndian.AppendUint32(b, v)
}

func decodeU32(b []byte) (uint32, []byte, error) {
	if len(b) < 4 {
		return 0, nil, io.ErrShortBuffer
	}
	return binary.LittleEndian.Uint32(b), b[4:], nil
}

func encodeU64(b []byte, v uint64) []byte {
	return binary.LittleEndian.AppendUint64(b, v)
}

func decodeU64(b []byte) (uint64, []byte, error) {
	if len(b) < 8 {
		return 0, nil, io.ErrShortBuffer
	}
	return binary.LittleEndian.Uint64(b), b[8:], nil
}

func encodeInt(b []byte, v int) []byte {
	return encodeU32(b, uint32(v))
}

func decodeInt(b []byte) (int, []byte, error) {
	v, b, err := decodeU32(b)
	return int(v), b, err
}

func encodeBytes(buffer []byte, b []byte) []byte {
	buffer = encodeU32(buffer, uint32(len(b)))
	return append(buffer, b...)
}

func decodeBytes(buffer []byte) ([]byte, []byte, error) {
	length, buffer, err := decodeU32(buffer)
	if err != nil {
		return nil, buffer, err
	}
	if uint32(len(buffer)) < length {
		return nil, buffer, io.ErrShortBuffer
	}
	return buffer[:length], buffer[length:], err
}

func encodeString(buffer []byte, s string) []byte {
	return encodeBytes(buffer, unsafe.Slice(unsafe.StringData(s), len(s)))
}

func decodeString(buffer []byte) (string, []byte, error) {
	result, buffer, err := decodeBytes(buffer)
	if err != nil || len(result) == 0 {
		return "", buffer, err
	}
	return unsafe.String(&result[0], len(result)), buffer, err
}

func encodeStrings(buffer []byte, args []string) []byte {
	buffer = encodeU32(buffer, uint32(len(args)))
	for _, arg := range args {
		buffer = encodeString(buffer, arg)
	}
	return buffer
}

func decodeStrings(buffer []byte, strings []string) (_ []string, _ []byte, err error) {
	var count uint32
	if count, buffer, err = decodeU32(buffer); err != nil {
		return
	}
	if uint32(cap(strings)) < count {
		strings = make([]string, count)
	} else {
		strings = strings[:count]
	}
	for i := uint32(0); i < count; i++ {
		strings[i], buffer, err = decodeString(buffer)
		if err != nil {
			return
		}
	}
	return strings, buffer, nil
}

func encodeIOVecs(buffer []byte, iovecs []IOVec) []byte {
	buffer = encodeU32(buffer, uint32(len(iovecs)))
	for _, iovec := range iovecs {
		buffer = encodeBytes(buffer, iovec)
	}
	return buffer
}

func encodeIOVecsPrefix(buffer []byte, iovecs []IOVec, size Size) []byte {
	if size == 0 {
		return encodeU32(buffer, 0)
	}
	prefixCount := 0
	prefixSize := 0
	for _, iovec := range iovecs {
		prefixCount++
		prefixSize += len(iovec)
		if Size(prefixSize) >= size {
			break
		}
	}
	buffer = encodeU32(buffer, uint32(prefixCount))
	remaining := size
	for _, iovec := range iovecs {
		if remaining <= Size(len(iovec)) {
			buffer = encodeBytes(buffer, iovec[:remaining])
			break
		}
		remaining -= Size(len(iovec))
		buffer = encodeBytes(buffer, iovec)
	}
	return buffer
}

func decodeIOVecs(buffer []byte, iovecs []IOVec) (_ []IOVec, _ []byte, err error) {
	var count uint32
	if count, buffer, err = decodeU32(buffer); err != nil {
		return
	}
	if uint32(cap(iovecs)) < count {
		iovecs = make([]IOVec, count)
	} else {
		iovecs = iovecs[:count]
	}
	for i := uint32(0); i < count; i++ {
		iovecs[i], buffer, err = decodeBytes(buffer)
		if err != nil {
			return
		}
	}
	return iovecs, buffer, nil
}

func encodeErrno(buffer []byte, errno Errno) []byte {
	return encodeU32(buffer, uint32(errno))
}

func decodeErrno(buffer []byte) (Errno, []byte, error) {
	errno, buffer, err := decodeU32(buffer)
	return Errno(errno), buffer, err
}

func encodeFD(buffer []byte, fd FD) []byte {
	return encodeU32(buffer, uint32(fd))
}

func decodeFD(buffer []byte) (FD, []byte, error) {
	fd, buffer, err := decodeU32(buffer)
	return FD(fd), buffer, err
}

func encodeClockID(buffer []byte, id ClockID) []byte {
	return encodeU32(buffer, uint32(id))
}

func decodeClockID(buffer []byte) (ClockID, []byte, error) {
	id, buffer, err := decodeU32(buffer)
	return ClockID(id), buffer, err
}

func encodeTimestamp(buffer []byte, id Timestamp) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeTimestamp(buffer []byte) (Timestamp, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return Timestamp(id), buffer, err
}

func encodeSize(buffer []byte, id Size) []byte {
	return encodeU32(buffer, uint32(id))
}

func decodeSize(buffer []byte) (Size, []byte, error) {
	id, buffer, err := decodeU32(buffer)
	return Size(id), buffer, err
}

func encodeFileType(buffer []byte, id FileType) []byte {
	return encodeU32(buffer, uint32(id))
}

func decodeFileType(buffer []byte) (FileType, []byte, error) {
	id, buffer, err := decodeU32(buffer)
	return FileType(id), buffer, err
}

func encodeFDFlags(buffer []byte, id FDFlags) []byte {
	return encodeU32(buffer, uint32(id))
}

func decodeFDFlags(buffer []byte) (FDFlags, []byte, error) {
	id, buffer, err := decodeU32(buffer)
	return FDFlags(id), buffer, err
}

func encodeFSTFlags(buffer []byte, id FSTFlags) []byte {
	return encodeU32(buffer, uint32(id))
}

func decodeFSTFlags(buffer []byte) (FSTFlags, []byte, error) {
	id, buffer, err := decodeU32(buffer)
	return FSTFlags(id), buffer, err
}

func encodeRights(buffer []byte, id Rights) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeRights(buffer []byte) (Rights, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return Rights(id), buffer, err
}

func encodeFileSize(buffer []byte, id FileSize) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeFileSize(buffer []byte) (FileSize, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return FileSize(id), buffer, err
}

func encodeDevice(buffer []byte, id Device) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeDevice(buffer []byte) (Device, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return Device(id), buffer, err
}

func encodeINode(buffer []byte, id INode) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeINode(buffer []byte) (INode, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return INode(id), buffer, err
}

func encodeLinkCount(buffer []byte, id LinkCount) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeLinkCount(buffer []byte) (LinkCount, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return LinkCount(id), buffer, err
}

func encodeAdvice(buffer []byte, id Advice) []byte {
	return encodeU32(buffer, uint32(id))
}

func decodeAdvice(buffer []byte) (Advice, []byte, error) {
	id, buffer, err := decodeU32(buffer)
	return Advice(id), buffer, err
}

func encodeExitCode(buffer []byte, id ExitCode) []byte {
	return encodeU32(buffer, uint32(id))
}

func decodeExitCode(buffer []byte) (ExitCode, []byte, error) {
	id, buffer, err := decodeU32(buffer)
	return ExitCode(id), buffer, err
}

func encodeSignal(buffer []byte, id Signal) []byte {
	return encodeU32(buffer, uint32(id))
}

func decodeSignal(buffer []byte) (Signal, []byte, error) {
	id, buffer, err := decodeU32(buffer)
	return Signal(id), buffer, err
}

func encodePreOpenType(buffer []byte, t PreOpenType) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodePreOpenType(buffer []byte) (PreOpenType, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return PreOpenType(t), buffer, err
}

func encodeUserData(buffer []byte, id UserData) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeUserData(buffer []byte) (UserData, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return UserData(id), buffer, err
}

func encodeEventType(buffer []byte, t EventType) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeEventType(buffer []byte) (EventType, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return EventType(t), buffer, err
}

func encodeSubscriptionClockFlags(buffer []byte, t SubscriptionClockFlags) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeSubscriptionClockFlags(buffer []byte) (SubscriptionClockFlags, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return SubscriptionClockFlags(t), buffer, err
}

func encodeEventFDReadWriteFlags(buffer []byte, t EventFDReadWriteFlags) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeEventFDReadWriteFlags(buffer []byte) (EventFDReadWriteFlags, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return EventFDReadWriteFlags(t), buffer, err
}

func encodeDirCookie(buffer []byte, id DirCookie) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeDirCookie(buffer []byte) (DirCookie, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return DirCookie(id), buffer, err
}

func encodeFileDelta(buffer []byte, id FileDelta) []byte {
	return encodeU64(buffer, uint64(id))
}

func decodeFileDelta(buffer []byte) (FileDelta, []byte, error) {
	id, buffer, err := decodeU64(buffer)
	return FileDelta(id), buffer, err
}

func encodeWhence(buffer []byte, t Whence) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeWhence(buffer []byte) (Whence, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return Whence(t), buffer, err
}

func encodeLookupFlags(buffer []byte, t LookupFlags) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeLookupFlags(buffer []byte) (LookupFlags, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return LookupFlags(t), buffer, err
}

func encodeOpenFlags(buffer []byte, t OpenFlags) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeOpenFlags(buffer []byte) (OpenFlags, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return OpenFlags(t), buffer, err
}

func encodeRIFlags(buffer []byte, t RIFlags) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeRIFlags(buffer []byte) (RIFlags, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return RIFlags(t), buffer, err
}

func encodeROFlags(buffer []byte, t ROFlags) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeROFlags(buffer []byte) (ROFlags, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return ROFlags(t), buffer, err
}

func encodeSIFlags(buffer []byte, t SIFlags) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeSIFlags(buffer []byte) (SIFlags, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return SIFlags(t), buffer, err
}

func encodeSDFlags(buffer []byte, t SDFlags) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeSDFlags(buffer []byte) (SDFlags, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return SDFlags(t), buffer, err
}

func encodeProtocolFamily(buffer []byte, t ProtocolFamily) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeProtocolFamily(buffer []byte) (ProtocolFamily, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return ProtocolFamily(t), buffer, err
}

func encodeSocketType(buffer []byte, t SocketType) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeSocketType(buffer []byte) (SocketType, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return SocketType(t), buffer, err
}

func encodeProtocol(buffer []byte, t Protocol) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeProtocol(buffer []byte) (Protocol, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return Protocol(t), buffer, err
}

func encodeSocketOptionLevel(buffer []byte, t SocketOptionLevel) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeSocketOptionLevel(buffer []byte) (SocketOptionLevel, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return SocketOptionLevel(t), buffer, err
}

func encodeSocketOption(buffer []byte, t SocketOption) []byte {
	return encodeU32(buffer, uint32(t))
}

func decodeSocketOption(buffer []byte) (SocketOption, []byte, error) {
	t, buffer, err := decodeU32(buffer)
	return SocketOption(t), buffer, err
}

func encodePreStat(buffer []byte, stat PreStat) []byte {
	buffer = encodePreOpenType(buffer, stat.Type)
	buffer = encodeSize(buffer, stat.PreStatDir.NameLength)
	return buffer
}

func decodePreStat(buffer []byte) (stat PreStat, _ []byte, err error) {
	if stat.Type, buffer, err = decodePreOpenType(buffer); err != nil {
		return
	}
	if stat.PreStatDir.NameLength, buffer, err = decodeSize(buffer); err != nil {
		return
	}
	return
}

func encodeFDStat(buffer []byte, stat FDStat) []byte {
	buffer = encodeFileType(buffer, stat.FileType)
	buffer = encodeFDFlags(buffer, stat.Flags)
	buffer = encodeRights(buffer, stat.RightsBase)
	buffer = encodeRights(buffer, stat.RightsInheriting)
	return buffer
}

func decodeFDStat(buffer []byte) (stat FDStat, _ []byte, err error) {
	if stat.FileType, buffer, err = decodeFileType(buffer); err != nil {
		return
	}
	if stat.Flags, buffer, err = decodeFDFlags(buffer); err != nil {
		return
	}
	if stat.RightsBase, buffer, err = decodeRights(buffer); err != nil {
		return
	}
	if stat.RightsInheriting, buffer, err = decodeRights(buffer); err != nil {
		return
	}
	return
}

func encodeFileStat(buffer []byte, stat FileStat) []byte {
	buffer = encodeDevice(buffer, stat.Device)
	buffer = encodeINode(buffer, stat.INode)
	buffer = encodeFileType(buffer, stat.FileType)
	buffer = encodeLinkCount(buffer, stat.NLink)
	buffer = encodeFileSize(buffer, stat.Size)
	buffer = encodeTimestamp(buffer, stat.AccessTime)
	buffer = encodeTimestamp(buffer, stat.ModifyTime)
	return encodeTimestamp(buffer, stat.ChangeTime)
}

func decodeFileStat(buffer []byte) (stat FileStat, _ []byte, err error) {
	if stat.Device, buffer, err = decodeDevice(buffer); err != nil {
		return
	}
	if stat.INode, buffer, err = decodeINode(buffer); err != nil {
		return
	}
	if stat.FileType, buffer, err = decodeFileType(buffer); err != nil {
		return
	}
	if stat.NLink, buffer, err = decodeLinkCount(buffer); err != nil {
		return
	}
	if stat.Size, buffer, err = decodeFileSize(buffer); err != nil {
		return
	}
	if stat.AccessTime, buffer, err = decodeTimestamp(buffer); err != nil {
		return
	}
	if stat.ModifyTime, buffer, err = decodeTimestamp(buffer); err != nil {
		return
	}
	stat.ChangeTime, buffer, err = decodeTimestamp(buffer)
	return
}

func encodeSubscription(buffer []byte, s Subscription) []byte {
	buffer = encodeUserData(buffer, s.UserData)
	buffer = encodeEventType(buffer, s.EventType)
	switch s.EventType {
	case FDReadEvent, FDWriteEvent:
		f := s.GetFDReadWrite()
		return encodeFD(buffer, f.FD)
	case ClockEvent:
		c := s.GetClock()
		buffer = encodeClockID(buffer, c.ID)
		buffer = encodeTimestamp(buffer, c.Precision)
		buffer = encodeTimestamp(buffer, c.Timeout)
		return encodeSubscriptionClockFlags(buffer, c.Flags)
	default:
		panic("invalid subscription event type")
	}
}

func decodeSubscription(buffer []byte) (s Subscription, _ []byte, err error) {
	if s.UserData, buffer, err = decodeUserData(buffer); err != nil {
		return
	}
	if s.EventType, buffer, err = decodeEventType(buffer); err != nil {
		return
	}
	switch s.EventType {
	case FDReadEvent, FDWriteEvent:
		var f SubscriptionFDReadWrite
		if f.FD, buffer, err = decodeFD(buffer); err != nil {
			return
		}
		s.SetFDReadWrite(f)
	case ClockEvent:
		var c SubscriptionClock
		if c.ID, buffer, err = decodeClockID(buffer); err != nil {
			return
		}
		if c.Precision, buffer, err = decodeTimestamp(buffer); err != nil {
			return
		}
		if c.Timeout, buffer, err = decodeTimestamp(buffer); err != nil {
			return
		}
		if c.Flags, buffer, err = decodeSubscriptionClockFlags(buffer); err != nil {
			return
		}
		s.SetClock(c)
	default:
		err = fmt.Errorf("invalid subscription event type: %v", s.EventType)
	}
	return s, buffer, err
}

func encodeEvent(buffer []byte, e Event) []byte {
	buffer = encodeUserData(buffer, e.UserData)
	buffer = encodeErrno(buffer, e.Errno)
	buffer = encodeEventType(buffer, e.EventType)
	switch e.EventType {
	case FDReadEvent, FDWriteEvent:
		buffer = encodeFileSize(buffer, e.FDReadWrite.NBytes)
		return encodeEventFDReadWriteFlags(buffer, e.FDReadWrite.Flags)
	case ClockEvent:
		return buffer
	default:
		panic("invalid event type")
	}
}

func decodeEvent(buffer []byte) (e Event, _ []byte, err error) {
	if e.UserData, buffer, err = decodeUserData(buffer); err != nil {
		return
	}
	if e.Errno, buffer, err = decodeErrno(buffer); err != nil {
		return
	}
	if e.EventType, buffer, err = decodeEventType(buffer); err != nil {
		return
	}
	switch e.EventType {
	case FDReadEvent, FDWriteEvent:
		if e.FDReadWrite.NBytes, buffer, err = decodeFileSize(buffer); err != nil {
			return
		}
		e.FDReadWrite.Flags, buffer, err = decodeEventFDReadWriteFlags(buffer)
	case ClockEvent:
	default:
		err = fmt.Errorf("invalid subscription event type: %v", e.EventType)
	}
	return e, buffer, err
}

func encodeSubscriptions(buffer []byte, subscriptions []Subscription) []byte {
	buffer = encodeU32(buffer, uint32(len(subscriptions)))
	for i := range subscriptions {
		buffer = encodeSubscription(buffer, subscriptions[i])
	}
	return buffer
}

func decodeSubscriptions(buffer []byte, subscriptions []Subscription) (_ []Subscription, _ []byte, err error) {
	var count uint32
	if count, buffer, err = decodeU32(buffer); err != nil {
		return
	}
	if uint32(cap(subscriptions)) < count {
		subscriptions = make([]Subscription, count)
	} else {
		subscriptions = subscriptions[:count]
	}
	for i := uint32(0); i < count; i++ {
		subscriptions[i], buffer, err = decodeSubscription(buffer)
		if err != nil {
			return
		}
	}
	return subscriptions, buffer, nil
}

func encodeEvents(buffer []byte, events []Event) []byte {
	buffer = encodeU32(buffer, uint32(len(events)))
	for i := range events {
		buffer = encodeEvent(buffer, events[i])
	}
	return buffer
}

func decodeEvents(buffer []byte, events []Event) (_ []Event, _ []byte, err error) {
	var count uint32
	if count, buffer, err = decodeU32(buffer); err != nil {
		return
	}
	if uint32(cap(events)) < count {
		events = make([]Event, count)
	} else {
		events = events[:count]
	}
	for i := uint32(0); i < count; i++ {
		events[i], buffer, err = decodeEvent(buffer)
		if err != nil {
			return
		}
	}
	return events, buffer, nil
}

func encodeDirEntry(buffer []byte, d DirEntry) []byte {
	buffer = encodeDirCookie(buffer, d.Next)
	buffer = encodeINode(buffer, d.INode)
	buffer = encodeFileType(buffer, d.Type)
	return encodeBytes(buffer, d.Name)
}

func decodeDirEntry(buffer []byte) (d DirEntry, _ []byte, err error) {
	if d.Next, buffer, err = decodeDirCookie(buffer); err != nil {
		return
	}
	if d.INode, buffer, err = decodeINode(buffer); err != nil {
		return
	}
	if d.Type, buffer, err = decodeFileType(buffer); err != nil {
		return
	}
	d.Name, buffer, err = decodeBytes(buffer)
	return d, buffer, err
}

func encodeDirEntries(buffer []byte, entries []DirEntry) []byte {
	buffer = encodeU32(buffer, uint32(len(entries)))
	for i := range entries {
		buffer = encodeDirEntry(buffer, entries[i])
	}
	return buffer
}

func decodeDirEntries(buffer []byte, entries []DirEntry) (_ []DirEntry, _ []byte, err error) {
	var count uint32
	if count, buffer, err = decodeU32(buffer); err != nil {
		return
	}
	if uint32(cap(entries)) < count {
		entries = make([]DirEntry, count)
	} else {
		entries = entries[:count]
	}
	for i := uint32(0); i < count; i++ {
		entries[i], buffer, err = decodeDirEntry(buffer)
		if err != nil {
			return
		}
	}
	return entries, buffer, nil
}

func encodeAddr(buffer []byte, addr SocketAddress) []byte {
	switch a := addr.(type) {
	case *Inet4Address:
		buffer = encodeProtocolFamily(buffer, Inet)
		buffer = encodeInt(buffer, a.Port)
		return encodeBytes(buffer, a.Addr[:])
	case *Inet6Address:
		buffer = encodeProtocolFamily(buffer, Inet6)
		buffer = encodeInt(buffer, a.Port)
		return encodeBytes(buffer, a.Addr[:])
	case *UnixAddress:
		panic("not implemented") // waiting for upstream support
	default:
		panic("unreachable")
	}
}

func decodeAddr(buffer []byte) (_ SocketAddress, _ []byte, err error) {
	var f ProtocolFamily
	if f, buffer, err = decodeProtocolFamily(buffer); err != nil {
		return
	}
	// TODO: eliminate these allocations by having the caller pass in a
	//  *Inet4Address and *Inet6Address to populate
	var ip []byte
	switch f {
	case Inet:
		var addr Inet4Address
		if addr.Port, buffer, err = decodeInt(buffer); err != nil {
			return
		}
		if ip, buffer, err = decodeBytes(buffer); err != nil {
			return
		}
		if len(ip) != 4 {
			return nil, buffer, fmt.Errorf("invalid IPv4 length: %v", len(ip))
		}
		copy(addr.Addr[:], ip)
		return &addr, buffer, nil
	case Inet6:
		var addr Inet6Address
		if addr.Port, buffer, err = decodeInt(buffer); err != nil {
			return
		}
		if ip, buffer, err = decodeBytes(buffer); err != nil {
			return
		}
		if len(ip) != 16 {
			return nil, buffer, fmt.Errorf("invalid IPv6 length: %v", len(ip))
		}
		copy(addr.Addr[:], ip)
		return &addr, buffer, nil
	default:
		return nil, buffer, fmt.Errorf("invalid or unsupported protocol family: %v", f)
	}
}
