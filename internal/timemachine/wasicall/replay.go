package wasicall

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	. "github.com/stealthrocket/wasi-go"
)

// Replayer implements wasi.System by replaying system calls recorded in a log.
//
// There are a few special cases to consider:
//   - a system call may be requested after the log has been replayed fully. In
//     this case, the replayer will call the eof callback with the pending
//     system call number and then forward the call (and all subsequent calls)
//     to the wasi.System it returns
//   - an error may occur when reading a record from the log or decoding its
//     contents. In this case, the error callback is called with a ReadError or
//     DecodeError
//   - a system call that doesn't match the next record in the log may be
//     requested. In this case, the error callback is called with a
//     UnexpectedSyscallError or one or more UnexpectedSyscallParamError errors
//   - if the ProcExit system call is called, the replayer will call the exit
//     callback rather than returning from the call
//
// The error and exit callbacks must not return normally, since these are
// called when an unrecoverable state is encountered. Rather, they should
// call panic(sys.NewExitError(...)) with an error code to halt execution of
// the WebAssembly module.
type Replayer struct {
	reader timemachine.RecordReader

	// Codec is used to encode and decode system call inputs and outputs.
	// It's not configurable at this time.
	codec Codec

	// Hooks that are called on edge cases.
	eof   func(Syscall) System
	error func(error)
	exit  func(ExitCode)

	// In strict mode, the Replayer will ensure that the system calls are
	// called with the same params as those stored on the records.
	strict bool

	// eofSystem is the wasi.System returned by the eof hook.
	eofSystem System
}

// var _ System = (*Replayer)(nil)
// var _ SocketsExtension = (*Replayer)(nil)

// NewReplayer creates a Replayer.
func NewReplayer(reader timemachine.RecordReader, eof func(Syscall) System, error func(error), exit func(ExitCode)) *Replayer {
	return &Replayer{
		reader: reader,
		eof:    eof,
		error:  error,
		exit:   exit,
		strict: true,
	}
}

func (r *Replayer) Preopen(hostfd int, path string, fdstat FDStat) FD {
	panic("Replayer cannot Preopen")
}

func (r *Replayer) Register(hostfd int, fdstat FDStat) FD {
	panic("Replayer cannot Register")
}

func (r *Replayer) isEOF(s Syscall, err error) (System, bool) {
	if err == io.EOF {
		if r.eofSystem == nil {
			r.eofSystem = r.eof(s)
		}
		return r.eofSystem, true
	}
	return nil, false
}

func (r *Replayer) handle(err error) {
	r.error(err)
	unreachable()
}

func (r *Replayer) ArgsGet(ctx context.Context) ([]string, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(ArgsGet, err); ok {
			return s.ArgsGet(ctx)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != ArgsGet {
		r.handle(&UnexpectedSyscallError{syscall, ArgsGet})
	}
	args, errno, err := r.codec.DecodeArgsGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	return args, errno
}

func (r *Replayer) EnvironGet(ctx context.Context) ([]string, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(EnvironGet, err); ok {
			return s.EnvironGet(ctx)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != EnvironGet {
		r.handle(&UnexpectedSyscallError{syscall, EnvironGet})
	}
	env, errno, err := r.codec.DecodeEnvironGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	return env, errno
}

func (r *Replayer) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(ClockResGet, err); ok {
			return s.ClockResGet(ctx, id)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != ClockResGet {
		r.handle(&UnexpectedSyscallError{syscall, ClockResGet})
	}
	recordID, timestamp, errno, err := r.codec.DecodeClockResGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if id != recordID {
			r.handle(&UnexpectedSyscallParamError{ClockResGet, "id", id, recordID})
		}
	}
	return timestamp, errno
}

func (r *Replayer) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(ClockTimeGet, err); ok {
			return s.ClockTimeGet(ctx, id, precision)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != ClockTimeGet {
		r.handle(&UnexpectedSyscallError{syscall, ClockTimeGet})
	}
	recordID, recordPrecision, timestamp, errno, err := r.codec.DecodeClockTimeGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return timestamp, errno
}

func (r *Replayer) FDAdvise(ctx context.Context, fd FD, offset FileSize, length FileSize, advice Advice) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDAdvise, err); ok {
			return s.FDAdvise(ctx, fd, offset, length, advice)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDAdvise {
		r.handle(&UnexpectedSyscallError{syscall, FDAdvise})
	}
	recordFD, recordOffset, recordLength, recordAdvice, errno, err := r.codec.DecodeFDAdvise(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) FDAllocate(ctx context.Context, fd FD, offset FileSize, length FileSize) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDAllocate, err); ok {
			return s.FDAllocate(ctx, fd, offset, length)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDAllocate {
		r.handle(&UnexpectedSyscallError{syscall, FDAllocate})
	}
	recordFD, recordOffset, recordLength, errno, err := r.codec.DecodeFDAllocate(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) FDClose(ctx context.Context, fd FD) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDClose, err); ok {
			return s.FDClose(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDClose {
		r.handle(&UnexpectedSyscallError{syscall, FDClose})
	}
	recordFD, errno, err := r.codec.DecodeFDClose(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if fd != recordFD {
			r.handle(&UnexpectedSyscallParamError{FDClose, "fd", fd, recordFD})
		}
	}
	return errno
}

func (r *Replayer) FDDataSync(ctx context.Context, fd FD) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDDataSync, err); ok {
			return s.FDDataSync(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDDataSync {
		r.handle(&UnexpectedSyscallError{syscall, FDDataSync})
	}
	recordFD, errno, err := r.codec.DecodeFDDataSync(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if fd != recordFD {
			r.handle(&UnexpectedSyscallParamError{FDDataSync, "fd", fd, recordFD})
		}
	}
	return errno
}

func (r *Replayer) FDStatGet(ctx context.Context, fd FD) (FDStat, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDStatGet, err); ok {
			return s.FDStatGet(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDStatGet {
		r.handle(&UnexpectedSyscallError{syscall, FDStatGet})
	}
	recordFD, stat, errno, err := r.codec.DecodeFDStatGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if fd != recordFD {
			r.handle(&UnexpectedSyscallParamError{FDStatGet, "fd", fd, recordFD})
		}
	}
	return stat, errno
}

func (r *Replayer) FDStatSetFlags(ctx context.Context, fd FD, flags FDFlags) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDStatSetFlags, err); ok {
			return s.FDStatSetFlags(ctx, fd, flags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDStatSetFlags {
		r.handle(&UnexpectedSyscallError{syscall, FDStatSetFlags})
	}
	recordFD, recordFlags, errno, err := r.codec.DecodeFDStatSetFlags(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) FDStatSetRights(ctx context.Context, fd FD, rightsBase, rightsInheriting Rights) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDStatSetRights, err); ok {
			return s.FDStatSetRights(ctx, fd, rightsBase, rightsInheriting)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDStatSetRights {
		r.handle(&UnexpectedSyscallError{syscall, FDStatSetRights})
	}
	recordFD, recordRightsBase, recordRightsInheriting, errno, err := r.codec.DecodeFDStatSetRights(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) FDFileStatGet(ctx context.Context, fd FD) (FileStat, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDFileStatGet, err); ok {
			return s.FDFileStatGet(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDFileStatGet {
		r.handle(&UnexpectedSyscallError{syscall, FDFileStatGet})
	}
	recordFD, stat, errno, err := r.codec.DecodeFDFileStatGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if fd != recordFD {
			r.handle(&UnexpectedSyscallParamError{FDFileStatGet, "fd", fd, recordFD})
		}
	}
	return stat, errno
}

func (r *Replayer) FDFileStatSetSize(ctx context.Context, fd FD, size FileSize) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDFileStatSetSize, err); ok {
			return s.FDFileStatSetSize(ctx, fd, size)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDFileStatSetSize {
		r.handle(&UnexpectedSyscallError{syscall, FDFileStatSetSize})
	}
	recordFD, recordSize, errno, err := r.codec.DecodeFDFileStatSetSize(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) FDFileStatSetTimes(ctx context.Context, fd FD, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDFileStatSetTimes, err); ok {
			return s.FDFileStatSetTimes(ctx, fd, accessTime, modifyTime, flags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDFileStatSetTimes {
		r.handle(&UnexpectedSyscallError{syscall, FDFileStatSetTimes})
	}
	recordFD, recordAccessTime, recordModifyTime, recordFlags, errno, err := r.codec.DecodeFDFileStatSetTimes(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) FDPread(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDPread, err); ok {
			return s.FDPread(ctx, fd, iovecs, offset)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDPread {
		r.handle(&UnexpectedSyscallError{syscall, FDPread})
	}
	recordFD, recordIOVecs, recordOffset, size, errno, err := r.codec.DecodeFDPread(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPread, "fd", fd, recordFD})
		}
		if !equalIovecShape(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPread, "iovecs", iovecs, recordIOVecs})
		}
		if offset != recordOffset {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPread, "offset", offset, recordOffset})
		}
		if len(mismatch) > 0 {
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replayer) FDPreStatGet(ctx context.Context, fd FD) (PreStat, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDPreStatGet, err); ok {
			return s.FDPreStatGet(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDPreStatGet {
		r.handle(&UnexpectedSyscallError{syscall, FDPreStatGet})
	}
	recordFD, stat, errno, err := r.codec.DecodeFDPreStatGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if fd != recordFD {
			r.handle(&UnexpectedSyscallParamError{FDPreStatGet, "fd", fd, recordFD})
		}
	}
	return stat, errno
}

func (r *Replayer) FDPreStatDirName(ctx context.Context, fd FD) (string, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDPreStatDirName, err); ok {
			return s.FDPreStatDirName(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDPreStatDirName {
		r.handle(&UnexpectedSyscallError{syscall, FDPreStatDirName})
	}
	recordFD, name, errno, err := r.codec.DecodeFDPreStatDirName(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if fd != recordFD {
			r.handle(&UnexpectedSyscallParamError{FDPreStatDirName, "fd", fd, recordFD})
		}
	}
	return name, errno
}

func (r *Replayer) FDPwrite(ctx context.Context, fd FD, iovecs []IOVec, offset FileSize) (Size, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDPwrite, err); ok {
			return s.FDPwrite(ctx, fd, iovecs, offset)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDPwrite {
		r.handle(&UnexpectedSyscallError{syscall, FDPwrite})
	}
	recordFD, recordIOVecs, recordOffset, size, errno, err := r.codec.DecodeFDPwrite(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPwrite, "fd", fd, recordFD})
		}
		if !equalIovec(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPwrite, "iovecs", iovecs, recordIOVecs})
		}
		if offset != recordOffset {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDPwrite, "offset", offset, recordOffset})
		}
		if len(mismatch) > 0 {
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replayer) FDRead(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDRead, err); ok {
			return s.FDRead(ctx, fd, iovecs)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDRead {
		r.handle(&UnexpectedSyscallError{syscall, FDRead})
	}
	recordFD, recordIOVecs, size, errno, err := r.codec.DecodeFDRead(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDRead, "fd", fd, recordFD})
		}
		if !equalIovecShape(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDRead, "iovecs", iovecs, recordIOVecs})
		}
		if len(mismatch) > 0 {
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replayer) FDReadDir(ctx context.Context, fd FD, entries []DirEntry, cookie DirCookie, bufferSizeBytes int) (int, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDReadDir, err); ok {
			return s.FDReadDir(ctx, fd, entries, cookie, bufferSizeBytes)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDReadDir {
		r.handle(&UnexpectedSyscallError{syscall, FDReadDir})
	}
	recordFD, recordEntries, recordCookie, recordBufferSizeBytes, count, errno, err := r.codec.DecodeFDReadDir(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDReadDir, "fd", fd, recordFD})
		}
		if len(entries) != len(recordEntries) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDReadDir, "entries", entries, recordEntries})
		}
		if cookie != recordCookie {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDReadDir, "cookie", cookie, recordCookie})
		}
		if bufferSizeBytes != recordBufferSizeBytes {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDReadDir, "bufferSizeBytes", bufferSizeBytes, recordBufferSizeBytes})
		}
		if len(mismatch) > 0 {
			r.handle(errors.Join(mismatch...))
		}
	}
	return count, errno
}

func (r *Replayer) FDRenumber(ctx context.Context, from, to FD) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDRenumber, err); ok {
			return s.FDRenumber(ctx, from, to)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDRenumber {
		r.handle(&UnexpectedSyscallError{syscall, FDRenumber})
	}
	recordFrom, recordTo, errno, err := r.codec.DecodeFDRenumber(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) FDSeek(ctx context.Context, fd FD, offset FileDelta, whence Whence) (FileSize, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDSeek, err); ok {
			return s.FDSeek(ctx, fd, offset, whence)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDSeek {
		r.handle(&UnexpectedSyscallError{syscall, FDSeek})
	}
	recordFD, recordOffset, recordWhence, size, errno, err := r.codec.DecodeFDSeek(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replayer) FDSync(ctx context.Context, fd FD) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDSync, err); ok {
			return s.FDSync(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDSync {
		r.handle(&UnexpectedSyscallError{syscall, FDSync})
	}
	recordFD, errno, err := r.codec.DecodeFDSync(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if fd != recordFD {
			r.handle(&UnexpectedSyscallParamError{FDSync, "fd", fd, recordFD})
		}
	}
	return errno
}

func (r *Replayer) FDTell(ctx context.Context, fd FD) (FileSize, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDTell, err); ok {
			return s.FDTell(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDTell {
		r.handle(&UnexpectedSyscallError{syscall, FDTell})
	}
	recordFD, size, errno, err := r.codec.DecodeFDTell(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		if fd != recordFD {
			r.handle(&UnexpectedSyscallParamError{FDTell, "fd", fd, recordFD})
		}
	}
	return size, errno
}

func (r *Replayer) FDWrite(ctx context.Context, fd FD, iovecs []IOVec) (Size, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(FDWrite, err); ok {
			return s.FDWrite(ctx, fd, iovecs)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != FDWrite {
		r.handle(&UnexpectedSyscallError{syscall, FDWrite})
	}
	recordFD, recordIOVecs, size, errno, err := r.codec.DecodeFDWrite(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDWrite, "fd", fd, recordFD})
		}
		if !equalIovec(iovecs, recordIOVecs) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{FDWrite, "iovecs", iovecs, recordIOVecs})
		}
		if len(mismatch) > 0 {
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replayer) Close(ctx context.Context) error {
	return nil
}

type ReadError struct{ error }

type DecodeError struct {
	Record *timemachine.Record
	error
}

type UnexpectedSyscallParamError struct {
	Syscall Syscall
	Name    string
	Actual  interface{}
	Expect  interface{}
}

func (e *UnexpectedSyscallParamError) Error() string {
	return fmt.Sprintf("expected %s.%s of %v, got %v", e.Syscall, e.Name, e.Expect, e.Actual)
}

type UnexpectedSyscallError struct {
	Actual Syscall
	Expect Syscall
}

func (e *UnexpectedSyscallError) Error() string {
	return fmt.Sprintf("expected syscall %s (%d) but got %s (%d)", e.Expect, int(e.Expect), e.Actual, int(e.Actual))
}

func equalIovecShape(a, b []IOVec) bool {
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

func equalIovec(a, b []IOVec) bool {
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

func unreachable() {
	panic("unreachable")
}
