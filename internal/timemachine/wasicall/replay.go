package wasicall

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	. "github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero/sys"
)

// Replay implements wasi.System by replaying system calls recorded in a log.
//
// During replay, records are pulled from the log and then used to answer
// system calls. No actual system calls are ever made. The WebAssembly module
// should thus be identical to the one used to create the log records.
//
// When the replay is complete, the implementation will return ENOSYS for
// subsequent system calls, and the EOF method will return true.
//
// When an error occurs, the replay will panic and the execution of the
// WebAssembly module will be halted. The following errors may occur:
//   - ReadError: there was an error reading from the log
//   - DecodeError: there was an error decoding a record from the log
//   - UnexpectedSyscallError: a system call was made that did not match the
//     next record in the log
//   - UnexpectedSyscallParamError: a system call was made with input that
//     did not match the next record in the log
//
// The error may also be a compound error, indicating that multiple errors
// were encountered. In this case, the error will implement
// interface{ Unwrap() []error }.
type Replay struct {
	reader timemachine.RecordReader

	// Codec is used to encode and decode system call inputs and outputs.
	// It's not configurable at this time.
	codec Codec

	// In strict mode, the replay will ensure that system calls are called
	// with the same params as those stored on the records. It's not
	// configurable at this time.
	//
	// TODO: consider separating the replay from validation, e.g. by making
	//  Codec an interface, and then having a ValidatingCodec wrapper that
	//  validates during decode
	strict bool

	// Cache for decoded slices.
	args          []string
	iovecs        []IOVec
	subscriptions []Subscription
	events        []Event
	entries       []DirEntry

	eof bool
}

var _ System = (*Replay)(nil)
var _ SocketsExtension = (*Replay)(nil)

// NewReplay creates a Replay.
func NewReplay(reader timemachine.RecordReader) *Replay {
	return &Replay{
		reader: reader,
		strict: true,
	}
}

func (r *Replay) Preopen(hostfd int, path string, fdstat FDStat) FD {
	panic("Replay cannot Preopen")
}

func (r *Replay) Register(hostfd int, fdstat FDStat) FD {
	panic("Replay cannot Register")
}

func (r *Replay) readRecord(syscall SyscallNumber) (*timemachine.Record, bool) {
	if r.eof {
		return nil, false
	}
	record, err := r.reader.ReadRecord()
	if err != nil {
		if err == io.EOF {
			r.eof = true
			return nil, false
		}
		panic(&ReadError{err})
	}
	if recordSyscall := SyscallNumber(record.FunctionID()); recordSyscall != syscall {
		panic(&UnexpectedSyscallError{recordSyscall, syscall})
	}
	return record, true
}

func (r *Replay) EOF() bool {
	return r.eof
}

func (r *Replay) ArgsGet(ctx context.Context) (args []string, errno Errno) {
	record, ok := r.readRecord(ArgsGet)
	if !ok {
		return nil, ENOSYS
	}
	var err error
	r.args, errno, err = r.codec.DecodeArgsGet(record.FunctionCall(), r.args[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	return r.args, errno
}

func (r *Replay) EnvironGet(ctx context.Context) (env []string, errno Errno) {
	record, ok := r.readRecord(EnvironGet)
	if !ok {
		return nil, ENOSYS
	}
	var err error
	r.args, errno, err = r.codec.DecodeEnvironGet(record.FunctionCall(), r.args[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	return r.args, errno
}

func (r *Replay) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	record, ok := r.readRecord(ClockResGet)
	if !ok {
		return 0, ENOSYS
	}
	recordID, timestamp, errno, err := r.codec.DecodeClockResGet(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && id != recordID {
		panic(&UnexpectedSyscallParamError{ClockResGet, "id", id, recordID})
	}
	return timestamp, errno
}

func (r *Replay) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	record, ok := r.readRecord(ClockTimeGet)
	if !ok {
		return 0, ENOSYS
	}
	recordID, recordPrecision, timestamp, errno, err := r.codec.DecodeClockTimeGet(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if id != recordID {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{ClockTimeGet, "id", id, recordID})
		}
		if precision != recordPrecision {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{ClockTimeGet, "precision", precision, recordPrecision})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return timestamp, errno
}

func (r *Replay) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	record, ok := r.readRecord(FDAdvise)
	if !ok {
		return ENOSYS
	}
	recordFD, recordOffset, recordLength, recordAdvice, errno, err := r.codec.DecodeFDAdvise(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDAdvise, "fd", fd, recordFD})
		}
		if offset != recordOffset {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDAdvise, "offset", offset, recordOffset})
		}
		if length != recordLength {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDAdvise, "length", length, recordLength})
		}
		if advice != recordAdvice {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDAdvise, "advice", advice, recordAdvice})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	record, ok := r.readRecord(FDAllocate)
	if !ok {
		return ENOSYS
	}
	recordFD, recordOffset, recordLength, errno, err := r.codec.DecodeFDAllocate(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDAllocate, "fd", fd, recordFD})
		}
		if offset != recordOffset {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDAllocate, "offset", offset, recordOffset})
		}
		if length != recordLength {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDAllocate, "length", length, recordLength})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) FDClose(ctx context.Context, fd FD) Errno {
	record, ok := r.readRecord(FDClose)
	if !ok {
		return ENOSYS
	}
	recordFD, errno, err := r.codec.DecodeFDClose(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{FDClose, "fd", fd, recordFD})
	}
	return errno
}

func (r *Replay) FDDataSync(ctx context.Context, fd FD) Errno {
	record, ok := r.readRecord(FDDataSync)
	if !ok {
		return ENOSYS
	}
	recordFD, errno, err := r.codec.DecodeFDDataSync(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{FDDataSync, "fd", fd, recordFD})
	}
	return errno
}

func (r *Replay) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	record, ok := r.readRecord(FDStatGet)
	if !ok {
		return FDStat{}, ENOSYS
	}
	recordFD, stat, errno, err := r.codec.DecodeFDStatGet(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{FDStatGet, "fd", fd, recordFD})
	}
	return stat, errno
}

func (r *Replay) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	record, ok := r.readRecord(FDStatSetFlags)
	if !ok {
		return ENOSYS
	}
	recordFD, recordFlags, errno, err := r.codec.DecodeFDStatSetFlags(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDStatSetFlags, "fd", fd, recordFD})
		}
		if flags != recordFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDStatSetFlags, "flags", flags, recordFlags})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) FDStatSetRights(ctx context.Context, fd FD, rightsBase, rightsInheriting Rights) Errno {
	record, ok := r.readRecord(FDStatSetRights)
	if !ok {
		return ENOSYS
	}
	recordFD, recordRightsBase, recordRightsInheriting, errno, err := r.codec.DecodeFDStatSetRights(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDStatSetRights, "fd", fd, recordFD})
		}
		if rightsBase != recordRightsBase {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDStatSetRights, "rightsBase", rightsBase, recordRightsBase})
		}
		if rightsInheriting != recordRightsInheriting {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDStatSetRights, "rightsInheriting", rightsInheriting, recordRightsInheriting})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	record, ok := r.readRecord(FDFileStatGet)
	if !ok {
		return FileStat{}, ENOSYS
	}
	recordFD, stat, errno, err := r.codec.DecodeFDFileStatGet(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{FDFileStatGet, "fd", fd, recordFD})
	}
	return stat, errno
}

func (r *Replay) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	record, ok := r.readRecord(FDFileStatSetSize)
	if !ok {
		return ENOSYS
	}
	recordFD, recordSize, errno, err := r.codec.DecodeFDFileStatSetSize(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDFileStatSetSize, "fd", fd, recordFD})
		}
		if size != recordSize {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDFileStatSetSize, "size", size, recordSize})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	record, ok := r.readRecord(FDFileStatSetTimes)
	if !ok {
		return ENOSYS
	}
	recordFD, recordAccessTime, recordModifyTime, recordFlags, errno, err := r.codec.DecodeFDFileStatSetTimes(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDFileStatSetTimes, "fd", fd, recordFD})
		}
		if accessTime != recordAccessTime {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDFileStatSetTimes, "accessTime", accessTime, recordAccessTime})
		}
		if modifyTime != recordModifyTime {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDFileStatSetTimes, "modifyTime", modifyTime, recordModifyTime})
		}
		if flags != recordFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDFileStatSetTimes, "flags", flags, recordFlags})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	record, ok := r.readRecord(FDPread)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordIOVecs, recordOffset, size, errno, err := r.codec.DecodeFDPread(record.FunctionCall(), r.iovecs[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.iovecs = recordIOVecs
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPread, "fd", fd, recordFD})
		}
		if !equalIovecsShape(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPread, "iovecs", iovecs, recordIOVecs})
		}
		if offset != recordOffset {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPread, "offset", offset, recordOffset})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replay) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	record, ok := r.readRecord(FDPreStatGet)
	if !ok {
		return PreStat{}, ENOSYS
	}
	recordFD, stat, errno, err := r.codec.DecodeFDPreStatGet(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{FDPreStatGet, "fd", fd, recordFD})
	}
	return stat, errno
}

func (r *Replay) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	record, ok := r.readRecord(FDPreStatDirName)
	if !ok {
		return "", ENOSYS
	}
	recordFD, name, errno, err := r.codec.DecodeFDPreStatDirName(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{FDPreStatDirName, "fd", fd, recordFD})
	}
	return name, errno
}

func (r *Replay) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	record, ok := r.readRecord(FDPwrite)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordIOVecs, recordOffset, size, errno, err := r.codec.DecodeFDPwrite(record.FunctionCall(), r.iovecs[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.iovecs = recordIOVecs
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPwrite, "fd", fd, recordFD})
		}
		if !equalIovecs(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPwrite, "iovecs", iovecs, recordIOVecs})
		}
		if offset != recordOffset {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPwrite, "offset", offset, recordOffset})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replay) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	record, ok := r.readRecord(FDRead)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordIOVecs, size, errno, err := r.codec.DecodeFDRead(record.FunctionCall(), r.iovecs[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.iovecs = recordIOVecs
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDRead, "fd", fd, recordFD})
		}
		if !equalIovecsShape(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDRead, "iovecs", iovecs, recordIOVecs})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	for i := range iovecs {
		copy(iovecs[i], recordIOVecs[i])
	}
	return size, errno
}

func (r *Replay) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	record, ok := r.readRecord(FDReadDir)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordEntries, recordCookie, recordBufferSizeBytes, count, errno, err := r.codec.DecodeFDReadDir(record.FunctionCall(), r.entries[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.entries = recordEntries
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDReadDir, "fd", fd, recordFD})
		}
		if len(entries) < len(recordEntries) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDReadDir, "entries", entries, recordEntries})
		}
		if cookie != recordCookie {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDReadDir, "cookie", cookie, recordCookie})
		}
		if bufferSizeBytes != recordBufferSizeBytes {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDReadDir, "bufferSizeBytes", bufferSizeBytes, recordBufferSizeBytes})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	copy(entries, recordEntries)
	return count, errno
}

func (r *Replay) FDRenumber(ctx context.Context, from, to FD) Errno {
	record, ok := r.readRecord(FDRenumber)
	if !ok {
		return ENOSYS
	}
	recordFrom, recordTo, errno, err := r.codec.DecodeFDRenumber(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if from != recordFrom {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDRenumber, "from", from, recordFrom})
		}
		if to != recordTo {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDRenumber, "to", to, recordTo})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	record, ok := r.readRecord(FDSeek)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordOffset, recordWhence, size, errno, err := r.codec.DecodeFDSeek(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDSeek, "fd", fd, recordFD})
		}
		if offset != recordOffset {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDSeek, "offset", offset, recordOffset})
		}
		if whence != recordWhence {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDSeek, "whence", whence, recordWhence})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replay) FDSync(ctx context.Context, fd FD) Errno {
	record, ok := r.readRecord(FDSync)
	if !ok {
		return ENOSYS
	}
	recordFD, errno, err := r.codec.DecodeFDSync(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{FDSync, "fd", fd, recordFD})
	}
	return errno
}

func (r *Replay) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	record, ok := r.readRecord(FDTell)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, size, errno, err := r.codec.DecodeFDTell(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{FDTell, "fd", fd, recordFD})
	}
	return size, errno
}

func (r *Replay) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	record, ok := r.readRecord(FDWrite)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordIOVecs, size, errno, err := r.codec.DecodeFDWrite(record.FunctionCall(), r.iovecs[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.iovecs = recordIOVecs
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDWrite, "fd", fd, recordFD})
		}
		if !equalIovecs(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDWrite, "iovecs", iovecs, recordIOVecs})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replay) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	record, ok := r.readRecord(PathCreateDirectory)
	if !ok {
		return ENOSYS
	}
	recordFD, recordPath, errno, err := r.codec.DecodePathCreateDirectory(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathCreateDirectory, "fd", fd, recordFD})
		}
		if path != recordPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathCreateDirectory, "path", path, recordPath})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	record, ok := r.readRecord(PathFileStatGet)
	if !ok {
		return FileStat{}, ENOSYS
	}
	recordFD, recordLookupFlags, recordPath, stat, errno, err := r.codec.DecodePathFileStatGet(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatGet, "fd", fd, recordFD})
		}
		if lookupFlags != recordLookupFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatGet, "lookupFlags", lookupFlags, recordLookupFlags})
		}
		if path != recordPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatGet, "path", path, recordPath})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return stat, errno
}

func (r *Replay) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	record, ok := r.readRecord(PathFileStatSetTimes)
	if !ok {
		return ENOSYS
	}
	recordFD, recordLookupFlags, recordPath, recordAccessTime, recordModifyTime, recordFlags, errno, err := r.codec.DecodePathFileStatSetTimes(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatSetTimes, "fd", fd, recordFD})
		}
		if lookupFlags != recordLookupFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatSetTimes, "lookupFlags", lookupFlags, recordLookupFlags})
		}
		if accessTime != recordAccessTime {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatSetTimes, "accessTime", accessTime, recordAccessTime})
		}
		if modifyTime != recordModifyTime {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatSetTimes, "modifyTime", modifyTime, recordModifyTime})
		}
		if flags != recordFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatSetTimes, "flags", flags, recordFlags})
		}
		if path != recordPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathFileStatSetTimes, "path", path, recordPath})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	record, ok := r.readRecord(PathLink)
	if !ok {
		return ENOSYS
	}
	recordOldFD, recordOldFlags, recordOldPath, recordNewFD, recordNewPath, errno, err := r.codec.DecodePathLink(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if oldFD != recordOldFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathLink, "oldFD", oldFD, recordOldFD})
		}
		if oldFlags != recordOldFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathLink, "oldFlags", oldFlags, recordOldFlags})
		}
		if oldPath != recordOldPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathLink, "oldPath", oldPath, recordOldPath})
		}
		if newFD != recordNewFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathLink, "newFD", newFD, recordNewFD})
		}
		if newPath != recordNewPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathLink, "newPath", newPath, recordNewPath})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	record, ok := r.readRecord(PathOpen)
	if !ok {
		return -1, ENOSYS
	}
	recordFD, recordDirFlags, recordPath, recordOpenFlags, recordRightsBase, recordRightsInheriting, recordFDFlags, newfd, errno, err := r.codec.DecodePathOpen(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathOpen, "fd", fd, recordFD})
		}
		if dirFlags != recordDirFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathOpen, "dirFlags", dirFlags, recordDirFlags})
		}
		if path != recordPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathOpen, "path", path, recordPath})
		}
		if openFlags != recordOpenFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathOpen, "openFlags", openFlags, recordOpenFlags})
		}
		if rightsBase != recordRightsBase {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathOpen, "rightsBase", rightsBase, recordRightsBase})
		}
		if rightsInheriting != recordRightsInheriting {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathOpen, "rightsInheriting", rightsInheriting, recordRightsInheriting})
		}
		if fdFlags != recordFDFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathOpen, "fdFlags", fdFlags, recordFDFlags})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return newfd, errno
}

func (r *Replay) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) ([]byte, Errno) {
	record, ok := r.readRecord(PathReadLink)
	if !ok {
		return nil, ENOSYS
	}
	recordFD, recordPath, result, errno, err := r.codec.DecodePathReadLink(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathReadLink, "fd", fd, recordFD})
		}
		if path != recordPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathReadLink, "path", path, recordPath})
		}
		if len(buffer) < len(result) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathReadLink, "buffer", buffer, result})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	copy(buffer, result)
	return buffer, errno
}

func (r *Replay) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	record, ok := r.readRecord(PathRemoveDirectory)
	if !ok {
		return ENOSYS
	}
	recordFD, recordPath, errno, err := r.codec.DecodePathRemoveDirectory(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathRemoveDirectory, "fd", fd, recordFD})
		}
		if path != recordPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathRemoveDirectory, "path", path, recordPath})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	record, ok := r.readRecord(PathRename)
	if !ok {
		return ENOSYS
	}
	recordFD, recordOldPath, recordNewFD, recordNewPath, errno, err := r.codec.DecodePathRename(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathRename, "fd", fd, recordFD})
		}
		if oldPath != recordOldPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathRename, "oldPath", oldPath, recordOldPath})
		}
		if newFD != recordNewFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathRename, "newFD", newFD, recordNewFD})
		}
		if newPath != recordNewPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathRename, "newPath", newPath, recordNewPath})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	record, ok := r.readRecord(PathSymlink)
	if !ok {
		return ENOSYS
	}
	recordOldPath, recordFD, recordNewPath, errno, err := r.codec.DecodePathSymlink(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if oldPath != recordOldPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathSymlink, "oldPath", oldPath, recordOldPath})
		}
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathSymlink, "fd", fd, recordFD})
		}
		if newPath != recordNewPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathSymlink, "newPath", newPath, recordNewPath})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	record, ok := r.readRecord(PathUnlinkFile)
	if !ok {
		return ENOSYS
	}
	recordFD, recordPath, errno, err := r.codec.DecodePathUnlinkFile(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathUnlinkFile, "fd", fd, recordFD})
		}
		if path != recordPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathUnlinkFile, "path", path, recordPath})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	record, ok := r.readRecord(PollOneOff)
	if !ok {
		return 0, ENOSYS
	}
	recordSubscriptions, recordEvents, count, errno, err := r.codec.DecodePollOneOff(record.FunctionCall(), r.subscriptions[:0], r.events[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.subscriptions = recordSubscriptions
	r.events = recordEvents
	if r.strict {
		var mismatch []error
		if !equalSubscriptions(subscriptions, recordSubscriptions) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PollOneOff, "subscriptions", subscriptions, recordSubscriptions})
		}
		if len(events) < len(recordEvents) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PollOneOff, "events", events, recordEvents})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	copy(events, recordEvents)
	return count, errno
}

func (r *Replay) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	record, ok := r.readRecord(ProcExit)
	if !ok {
		return ENOSYS
	}
	recordExitCode, errno, err := r.codec.DecodeProcExit(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && exitCode != recordExitCode {
		panic(&UnexpectedSyscallParamError{ProcExit, "exitCode", exitCode, recordExitCode})
	}
	_ = errno
	panic(sys.NewExitError(uint32(exitCode)))
}

func (r *Replay) ProcRaise(ctx context.Context, signal Signal) Errno {
	record, ok := r.readRecord(ProcRaise)
	if !ok {
		return ENOSYS
	}
	recordSignal, errno, err := r.codec.DecodeProcRaise(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && signal != recordSignal {
		panic(&UnexpectedSyscallParamError{ProcRaise, "signal", signal, recordSignal})
	}
	return errno
}

func (r *Replay) SchedYield(ctx context.Context) Errno {
	record, ok := r.readRecord(SchedYield)
	if !ok {
		return ENOSYS
	}
	errno, err := r.codec.DecodeSchedYield(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	return errno
}

func (r *Replay) RandomGet(ctx context.Context, buffer []byte) Errno {
	record, ok := r.readRecord(RandomGet)
	if !ok {
		return ENOSYS
	}
	recordBuffer, errno, err := r.codec.DecodeRandomGet(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && len(buffer) != len(recordBuffer) {
		panic(&UnexpectedSyscallParamError{RandomGet, "buffer", buffer, recordBuffer})
	}
	copy(buffer, recordBuffer)
	return errno
}

func (r *Replay) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, Errno) {
	record, ok := r.readRecord(SockAccept)
	if !ok {
		return -1, ENOSYS
	}
	recordFD, recordFlags, newfd, errno, err := r.codec.DecodeSockAccept(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockAccept, "fd", fd, recordFD})
		}
		if flags != recordFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockAccept, "flags", flags, recordFlags})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return newfd, errno
}

func (r *Replay) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	record, ok := r.readRecord(SockShutdown)
	if !ok {
		return ENOSYS
	}
	recordFD, recordFlags, errno, err := r.codec.DecodeSockShutdown(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockShutdown, "fd", fd, recordFD})
		}
		if flags != recordFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockShutdown, "flags", flags, recordFlags})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, Errno) {
	record, ok := r.readRecord(SockRecv)
	if !ok {
		return 0, 0, ENOSYS
	}
	recordFD, recordIOVecs, recordIFlags, size, oflags, errno, err := r.codec.DecodeSockRecv(record.FunctionCall(), r.iovecs[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.iovecs = recordIOVecs
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockRecv, "fd", fd, recordFD})
		}
		if !equalIovecsShape(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockRecv, "iovecs", iovecs, recordIOVecs})
		}
		if iflags != recordIFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockRecv, "iflags", iflags, recordIFlags})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return size, oflags, errno
}

func (r *Replay) SockSend(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags) (Size, Errno) {
	record, ok := r.readRecord(SockSend)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordIOVecs, recordIFlags, size, errno, err := r.codec.DecodeSockSend(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSend, "fd", fd, recordFD})
		}
		if !equalIovecs(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSend, "iovecs", iovecs, recordIOVecs})
		}
		if iflags != recordIFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSend, "iflags", iflags, recordIFlags})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replay) SockOpen(ctx context.Context, protocolFamily ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights) (FD, Errno) {
	record, ok := r.readRecord(SockOpen)
	if !ok {
		return -1, ENOSYS
	}
	recordProtocolFamily, recordSocketType, recordProtocol, recordRightsBase, recordRightsInheriting, newfd, errno, err := r.codec.DecodeSockOpen(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if protocolFamily != recordProtocolFamily {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockOpen, "protocolFamily", protocolFamily, recordProtocolFamily})
		}
		if socketType != recordSocketType {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockOpen, "socketType", socketType, recordSocketType})
		}
		if protocol != recordProtocol {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockOpen, "protocol", protocol, recordProtocol})
		}
		if rightsBase != recordRightsBase {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockOpen, "rightsBase", rightsBase, recordRightsBase})
		}
		if rightsInheriting != recordRightsInheriting {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockOpen, "rightsInheriting", rightsInheriting, recordRightsInheriting})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return newfd, errno
}

func (r *Replay) SockBind(ctx context.Context, fd FD, addr SocketAddress) Errno {
	record, ok := r.readRecord(SockBind)
	if !ok {
		return ENOSYS
	}
	recordFD, recordAddr, errno, err := r.codec.DecodeSockBind(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockBind, "fd", fd, recordFD})
		}
		if addr != recordAddr {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockBind, "addr", addr, recordAddr})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) SockConnect(ctx context.Context, fd FD, addr SocketAddress) Errno {
	record, ok := r.readRecord(SockConnect)
	if !ok {
		return ENOSYS
	}
	recordFD, recordAddr, errno, err := r.codec.DecodeSockConnect(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockConnect, "fd", fd, recordFD})
		}
		if addr != recordAddr {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockConnect, "addr", addr, recordAddr})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) SockListen(ctx context.Context, fd FD, backlog int) Errno {
	record, ok := r.readRecord(SockListen)
	if !ok {
		return ENOSYS
	}
	recordFD, recordBacklog, errno, err := r.codec.DecodeSockListen(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockListen, "fd", fd, recordFD})
		}
		if backlog != recordBacklog {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockListen, "backlog", backlog, recordBacklog})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress) (Size, Errno) {
	record, ok := r.readRecord(SockSendTo)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordIOVecs, recordIFlags, recordAddr, size, errno, err := r.codec.DecodeSockSendTo(record.FunctionCall(), r.iovecs[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.iovecs = recordIOVecs
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSendTo, "fd", fd, recordFD})
		}
		if !equalIovecs(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSendTo, "iovecs", iovecs, recordIOVecs})
		}
		if iflags != recordIFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSendTo, "iflags", iflags, recordIFlags})
		}
		if addr != recordAddr {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSendTo, "addr", addr, recordAddr})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replay) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, SocketAddress, Errno) {
	record, ok := r.readRecord(SockRecvFrom)
	if !ok {
		return 0, 0, nil, ENOSYS
	}
	recordFD, recordIOVecs, recordIFlags, size, oflags, addr, errno, err := r.codec.DecodeSockRecvFrom(record.FunctionCall(), r.iovecs[:0])
	if err != nil {
		panic(&DecodeError{record, err})
	}
	r.iovecs = recordIOVecs
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockRecvFrom, "fd", fd, recordFD})
		}
		if !equalIovecsShape(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockRecvFrom, "iovecs", iovecs, recordIOVecs})
		}
		if iflags != recordIFlags {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockRecvFrom, "iflags", iflags, recordIFlags})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return size, oflags, addr, errno
}

func (r *Replay) SockGetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (int, Errno) {
	record, ok := r.readRecord(SockGetOptInt)
	if !ok {
		return 0, ENOSYS
	}
	recordFD, recordLevel, recordOption, value, errno, err := r.codec.DecodeSockGetOptInt(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockGetOptInt, "fd", fd, recordFD})
		}
		if level != recordLevel {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockGetOptInt, "level", level, recordLevel})
		}
		if option != recordOption {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockGetOptInt, "option", option, recordOption})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return value, errno
}

func (r *Replay) SockSetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value int) Errno {
	record, ok := r.readRecord(SockSetOptInt)
	if !ok {
		return ENOSYS
	}
	recordFD, recordLevel, recordOption, recordValue, errno, err := r.codec.DecodeSockSetOptInt(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSetOptInt, "fd", fd, recordFD})
		}
		if level != recordLevel {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSetOptInt, "level", level, recordLevel})
		}
		if option != recordOption {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSetOptInt, "option", option, recordOption})
		}
		if value != recordValue {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{SockSetOptInt, "value", value, recordValue})
		}
		if len(mismatch) > 0 {
			panic(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replay) SockLocalAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	record, ok := r.readRecord(SockLocalAddress)
	if !ok {
		return nil, ENOSYS
	}
	recordFD, addr, errno, err := r.codec.DecodeSockLocalAddress(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{SockLocalAddress, "fd", fd, recordFD})
	}
	return addr, errno
}

func (r *Replay) SockPeerAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	record, ok := r.readRecord(SockPeerAddress)
	if !ok {
		return nil, ENOSYS
	}
	recordFD, addr, errno, err := r.codec.DecodeSockPeerAddress(record.FunctionCall())
	if err != nil {
		panic(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		panic(&UnexpectedSyscallParamError{SockPeerAddress, "fd", fd, recordFD})
	}
	return addr, errno
}

func (r *Replay) Close(ctx context.Context) error {
	return nil
}

type ReadError struct{ error }

type DecodeError struct {
	Record *timemachine.Record
	error
}

type UnexpectedSyscallParamError struct {
	Syscall SyscallNumber
	Name    string
	Actual  interface{}
	Expect  interface{}
}

func (e *UnexpectedSyscallParamError) Error() string {
	return fmt.Sprintf("expected %s.%s of %v, got %v", e.Syscall, e.Name, e.Expect, e.Actual)
}

type UnexpectedSyscallError struct {
	Recorded SyscallNumber
	Observed SyscallNumber
}

func (e *UnexpectedSyscallError) Error() string {
	return fmt.Sprintf("got syscall %s (%d) but log contained %s (%d)", e.Observed, int(e.Observed), e.Recorded, int(e.Recorded))
}

func equalIovecsShape(a, b []IOVec) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
	}
	return true
}

func equalIovecs(a, b []IOVec) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !bytes.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}

func equalSubscriptions(a, b []Subscription) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !equalSubscription(a[i], b[i]) {
			return false
		}
	}
	return true
}

func equalSubscription(a, b Subscription) bool {
	if a.EventType != b.EventType {
		return false
	}
	if a.UserData != b.UserData {
		return false
	}
	if a.EventType == ClockEvent {
		return a.GetClock() == b.GetClock()
	} else if a.EventType == FDReadEvent || a.EventType == FDWriteEvent {
		return a.GetFDReadWrite() == b.GetFDReadWrite()
	} else {
		return false // invalid event type; cannot compare
	}
}
