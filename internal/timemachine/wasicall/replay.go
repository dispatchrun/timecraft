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

const (
	EOFExitCode   = 0xE0F
	ErrorExitCode = 0xBAD
)

// ReplayController controls a replay.
type ReplayController interface {
	// EOF is called when the log has been replayed fully and then another
	// system call is made.
	//
	// If the controller returns a new wasi.System here, the pending system
	// call and all subsequent system calls will be forwarded to it. If the
	// controller returns nil, the WebAssembly module is terminated with
	// EOFExitCode.
	EOF(pendingSyscall Syscall) System

	// Error is called when the replay encounters an error from which it
	// cannot recover.
	//
	// The error will be one of:
	// - ReadError: there was an error reading from the log
	// - DecodeError: there was an error decoding a record from the log
	// - UnexpectedSyscallError: a system call was made that did not match the
	//   next record in the log
	// - UnexpectedSyscallParamError: a system call was made with input that
	//   did not match the next record in the log
	//
	// The error may also be a compound error, indicating that multiple errors
	// were encountered. In this case, the error will implement
	// interface{ Unwrap() []error }.
	//
	// After the controller has been notified, the WebAssembly module is
	// terminated with ErrorExitCode.
	Error(err error)
}

// Replayer implements wasi.System by replaying system calls recorded in a log.
type Replayer struct {
	reader     timemachine.RecordReader
	controller ReplayController

	// Codec is used to encode and decode system call inputs and outputs.
	// It's not configurable at this time.
	codec Codec

	// In strict mode, the Replayer will ensure that the system calls are
	// called with the same params as those stored on the records. It's not
	// configurable at this time.
	strict bool

	// eofSystem is the wasi.System returned by the EOF hook.
	eofSystem System

	// Cache for decoded slices.
	args          []string
	iovecs        []IOVec
	subscriptions []Subscription
	events        []Event
}

var _ System = (*Replayer)(nil)
var _ SocketsExtension = (*Replayer)(nil)

// NewReplayer creates a Replayer.
func NewReplayer(reader timemachine.RecordReader, controller ReplayController) *Replayer {
	return &Replayer{
		reader:     reader,
		controller: controller,
		strict:     true,
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
			r.eofSystem = r.controller.EOF(s)
			if r.eofSystem == nil {
				panic(sys.NewExitError(EOFExitCode))
			}
		}
		return r.eofSystem, true
	}
	return nil, false
}

func (r *Replayer) handle(err error) {
	r.controller.Error(err)
	panic(sys.NewExitError(ErrorExitCode))
}

func (r *Replayer) ArgsGet(ctx context.Context) (args []string, errno Errno) {
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
	r.args, errno, err = r.codec.DecodeArgsGet(record.FunctionCall(), r.args[:0])
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	return r.args, errno
}

func (r *Replayer) EnvironGet(ctx context.Context) (env []string, errno Errno) {
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
	r.args, errno, err = r.codec.DecodeEnvironGet(record.FunctionCall(), r.args[:0])
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	return r.args, errno
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
	if r.strict && id != recordID {
		r.handle(&UnexpectedSyscallParamError{ClockResGet, "id", id, recordID})
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
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{FDClose, "fd", fd, recordFD})
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
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{FDDataSync, "fd", fd, recordFD})
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
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{FDStatGet, "fd", fd, recordFD})
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
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{FDFileStatGet, "fd", fd, recordFD})
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
		if !equalIovecsShape(iovecs, recordIOVecs) {
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
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{FDPreStatGet, "fd", fd, recordFD})
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
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{FDPreStatDirName, "fd", fd, recordFD})
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
		if !equalIovecs(iovecs, recordIOVecs) {
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
		if !equalIovecsShape(iovecs, recordIOVecs) {
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
			r.handle(errors.Join(mismatch...))
		}
	}
	copy(entries, recordEntries)
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
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{FDSync, "fd", fd, recordFD})
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
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{FDTell, "fd", fd, recordFD})
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
	recordFD, recordIOVecs, size, errno, err := r.codec.DecodeFDWrite(record.FunctionCall(), r.iovecs[:0])
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replayer) PathCreateDirectory(ctx context.Context, fd FD, path string) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathCreateDirectory, err); ok {
			return s.PathCreateDirectory(ctx, fd, path)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathCreateDirectory {
		r.handle(&UnexpectedSyscallError{syscall, PathCreateDirectory})
	}
	recordFD, recordPath, errno, err := r.codec.DecodePathCreateDirectory(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) PathFileStatGet(ctx context.Context, fd FD, lookupFlags LookupFlags, path string) (FileStat, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathFileStatGet, err); ok {
			return s.PathFileStatGet(ctx, fd, lookupFlags, path)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathFileStatGet {
		r.handle(&UnexpectedSyscallError{syscall, PathFileStatGet})
	}
	recordFD, recordLookupFlags, recordPath, stat, errno, err := r.codec.DecodePathFileStatGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return stat, errno
}

func (r *Replayer) PathFileStatSetTimes(ctx context.Context, fd FD, lookupFlags LookupFlags, path string, accessTime, modifyTime Timestamp, flags FSTFlags) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathFileStatSetTimes, err); ok {
			return s.PathFileStatSetTimes(ctx, fd, lookupFlags, path, accessTime, modifyTime, flags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathFileStatSetTimes {
		r.handle(&UnexpectedSyscallError{syscall, PathFileStatSetTimes})
	}
	recordFD, recordLookupFlags, recordPath, recordAccessTime, recordModifyTime, recordFlags, errno, err := r.codec.DecodePathFileStatSetTimes(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) PathLink(ctx context.Context, oldFD FD, oldFlags LookupFlags, oldPath string, newFD FD, newPath string) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathLink, err); ok {
			return s.PathLink(ctx, oldFD, oldFlags, oldPath, newFD, newPath)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathLink {
		r.handle(&UnexpectedSyscallError{syscall, PathLink})
	}
	recordOldFD, recordOldFlags, recordOldPath, recordNewFD, recordNewPath, errno, err := r.codec.DecodePathLink(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) PathOpen(ctx context.Context, fd FD, dirFlags LookupFlags, path string, openFlags OpenFlags, rightsBase, rightsInheriting Rights, fdFlags FDFlags) (FD, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathOpen, err); ok {
			return s.PathOpen(ctx, fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathOpen {
		r.handle(&UnexpectedSyscallError{syscall, PathOpen})
	}
	recordFD, recordDirFlags, recordPath, recordOpenFlags, recordRightsBase, recordRightsInheriting, recordFDFlags, newfd, errno, err := r.codec.DecodePathOpen(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return newfd, errno
}

func (r *Replayer) PathReadLink(ctx context.Context, fd FD, path string, buffer []byte) ([]byte, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathReadLink, err); ok {
			return s.PathReadLink(ctx, fd, path, buffer)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathReadLink {
		r.handle(&UnexpectedSyscallError{syscall, PathReadLink})
	}
	recordFD, recordPath, recordBuffer, result, errno, err := r.codec.DecodePathReadLink(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict {
		var mismatch []error
		if fd != recordFD {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathReadLink, "fd", fd, recordFD})
		}
		if path != recordPath {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathReadLink, "path", path, recordPath})
		}
		if len(buffer) != len(recordBuffer) {
			mismatch = append(mismatch, &UnexpectedSyscallParamError{PathReadLink, "buffer", buffer, recordFD})
		}
		if len(mismatch) > 0 {
			r.handle(errors.Join(mismatch...))
		}
	}
	copy(buffer, result)
	return buffer, errno
}

func (r *Replayer) PathRemoveDirectory(ctx context.Context, fd FD, path string) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathRemoveDirectory, err); ok {
			return s.PathRemoveDirectory(ctx, fd, path)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathRemoveDirectory {
		r.handle(&UnexpectedSyscallError{syscall, PathRemoveDirectory})
	}
	recordFD, recordPath, errno, err := r.codec.DecodePathRemoveDirectory(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) PathRename(ctx context.Context, fd FD, oldPath string, newFD FD, newPath string) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathRename, err); ok {
			return s.PathRename(ctx, fd, oldPath, newFD, newPath)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathRename {
		r.handle(&UnexpectedSyscallError{syscall, PathRename})
	}
	recordFD, recordOldPath, recordNewFD, recordNewPath, errno, err := r.codec.DecodePathRename(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) PathSymlink(ctx context.Context, oldPath string, fd FD, newPath string) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathSymlink, err); ok {
			return s.PathSymlink(ctx, oldPath, fd, newPath)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathSymlink {
		r.handle(&UnexpectedSyscallError{syscall, PathSymlink})
	}
	recordOldPath, recordFD, recordNewPath, errno, err := r.codec.DecodePathSymlink(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) PathUnlinkFile(ctx context.Context, fd FD, path string) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PathUnlinkFile, err); ok {
			return s.PathUnlinkFile(ctx, fd, path)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PathUnlinkFile {
		r.handle(&UnexpectedSyscallError{syscall, PathUnlinkFile})
	}
	recordFD, recordPath, errno, err := r.codec.DecodePathUnlinkFile(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) PollOneOff(ctx context.Context, subscriptions []Subscription, events []Event) (int, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(PollOneOff, err); ok {
			return s.PollOneOff(ctx, subscriptions, events)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != PollOneOff {
		r.handle(&UnexpectedSyscallError{syscall, PollOneOff})
	}
	recordSubscriptions, recordEvents, count, errno, err := r.codec.DecodePollOneOff(record.FunctionCall(), r.subscriptions[:0], r.events[:0])
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	copy(events, recordEvents)
	return count, errno
}

func (r *Replayer) ProcExit(ctx context.Context, exitCode ExitCode) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(ProcExit, err); ok {
			return s.ProcExit(ctx, exitCode)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != ProcExit {
		r.handle(&UnexpectedSyscallError{syscall, ProcExit})
	}
	recordExitCode, errno, err := r.codec.DecodeProcExit(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict && exitCode != recordExitCode {
		r.handle(&UnexpectedSyscallParamError{ProcExit, "exitCode", exitCode, recordExitCode})
	}
	_ = errno
	panic(sys.NewExitError(uint32(exitCode)))
}

func (r *Replayer) ProcRaise(ctx context.Context, signal Signal) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(ProcRaise, err); ok {
			return s.ProcRaise(ctx, signal)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != ProcRaise {
		r.handle(&UnexpectedSyscallError{syscall, ProcRaise})
	}
	recordSignal, errno, err := r.codec.DecodeProcRaise(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict && signal != recordSignal {
		r.handle(&UnexpectedSyscallParamError{ProcRaise, "signal", signal, recordSignal})
	}
	return errno
}

func (r *Replayer) SchedYield(ctx context.Context) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SchedYield, err); ok {
			return s.SchedYield(ctx)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SchedYield {
		r.handle(&UnexpectedSyscallError{syscall, SchedYield})
	}
	errno, err := r.codec.DecodeSchedYield(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	return errno
}

func (r *Replayer) RandomGet(ctx context.Context, buffer []byte) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(RandomGet, err); ok {
			return s.RandomGet(ctx, buffer)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != RandomGet {
		r.handle(&UnexpectedSyscallError{syscall, RandomGet})
	}
	recordBuffer, errno, err := r.codec.DecodeRandomGet(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict && len(buffer) != len(recordBuffer) {
		r.handle(&UnexpectedSyscallParamError{RandomGet, "buffer", buffer, recordBuffer})
	}
	copy(buffer, recordBuffer)
	return errno
}

func (r *Replayer) SockAccept(ctx context.Context, fd FD, flags FDFlags) (FD, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockAccept, err); ok {
			return s.SockAccept(ctx, fd, flags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockAccept {
		r.handle(&UnexpectedSyscallError{syscall, SockAccept})
	}
	recordFD, recordFlags, newfd, errno, err := r.codec.DecodeSockAccept(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return newfd, errno
}

func (r *Replayer) SockShutdown(ctx context.Context, fd FD, flags SDFlags) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockShutdown, err); ok {
			return s.SockShutdown(ctx, fd, flags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockShutdown {
		r.handle(&UnexpectedSyscallError{syscall, SockShutdown})
	}
	recordFD, recordFlags, errno, err := r.codec.DecodeSockShutdown(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) SockRecv(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockRecv, err); ok {
			return s.SockRecv(ctx, fd, iovecs, iflags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockRecv {
		r.handle(&UnexpectedSyscallError{syscall, SockRecv})
	}
	recordFD, recordIOVecs, recordIFlags, size, oflags, errno, err := r.codec.DecodeSockRecv(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, oflags, errno
}

func (r *Replayer) SockSend(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags) (Size, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockSend, err); ok {
			return s.SockSend(ctx, fd, iovecs, iflags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockSend {
		r.handle(&UnexpectedSyscallError{syscall, SockSend})
	}
	recordFD, recordIOVecs, recordIFlags, size, errno, err := r.codec.DecodeSockSend(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replayer) SockOpen(ctx context.Context, protocolFamily ProtocolFamily, socketType SocketType, protocol Protocol, rightsBase, rightsInheriting Rights) (FD, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockOpen, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return -1, ENOSYS
			}
			return se.SockOpen(ctx, protocolFamily, socketType, protocol, rightsBase, rightsInheriting)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockOpen {
		r.handle(&UnexpectedSyscallError{syscall, SockOpen})
	}
	recordProtocolFamily, recordSocketType, recordProtocol, recordRightsBase, recordRightsInheriting, newfd, errno, err := r.codec.DecodeSockOpen(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return newfd, errno
}

func (r *Replayer) SockBind(ctx context.Context, fd FD, addr SocketAddress) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockBind, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return ENOSYS
			}
			return se.SockBind(ctx, fd, addr)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockBind {
		r.handle(&UnexpectedSyscallError{syscall, SockBind})
	}
	recordFD, recordAddr, errno, err := r.codec.DecodeSockBind(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) SockConnect(ctx context.Context, fd FD, addr SocketAddress) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockConnect, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return ENOSYS
			}
			return se.SockConnect(ctx, fd, addr)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockConnect {
		r.handle(&UnexpectedSyscallError{syscall, SockConnect})
	}
	recordFD, recordAddr, errno, err := r.codec.DecodeSockConnect(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) SockListen(ctx context.Context, fd FD, backlog int) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockListen, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return ENOSYS
			}
			return se.SockListen(ctx, fd, backlog)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockListen {
		r.handle(&UnexpectedSyscallError{syscall, SockListen})
	}
	recordFD, recordBacklog, errno, err := r.codec.DecodeSockListen(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) SockSendTo(ctx context.Context, fd FD, iovecs []IOVec, iflags SIFlags, addr SocketAddress) (Size, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockSendTo, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return 0, ENOSYS
			}
			return se.SockSendTo(ctx, fd, iovecs, iflags, addr)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockSendTo {
		r.handle(&UnexpectedSyscallError{syscall, SockSendTo})
	}
	recordFD, recordIOVecs, recordIFlags, recordAddr, size, errno, err := r.codec.DecodeSockSendTo(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, errno
}

func (r *Replayer) SockRecvFrom(ctx context.Context, fd FD, iovecs []IOVec, iflags RIFlags) (Size, ROFlags, SocketAddress, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockRecvFrom, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return 0, 0, nil, ENOSYS
			}
			return se.SockRecvFrom(ctx, fd, iovecs, iflags)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockRecvFrom {
		r.handle(&UnexpectedSyscallError{syscall, SockRecvFrom})
	}
	recordFD, recordIOVecs, recordIFlags, size, oflags, addr, errno, err := r.codec.DecodeSockRecvFrom(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return size, oflags, addr, errno
}

func (r *Replayer) SockGetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption) (int, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockGetOptInt, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return 0, ENOSYS
			}
			return se.SockGetOptInt(ctx, fd, level, option)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockGetOptInt {
		r.handle(&UnexpectedSyscallError{syscall, SockGetOptInt})
	}
	recordFD, recordLevel, recordOption, value, errno, err := r.codec.DecodeSockGetOptInt(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return value, errno
}

func (r *Replayer) SockSetOptInt(ctx context.Context, fd FD, level SocketOptionLevel, option SocketOption, value int) Errno {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockSetOptInt, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return ENOSYS
			}
			return se.SockSetOptInt(ctx, fd, level, option, value)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockSetOptInt {
		r.handle(&UnexpectedSyscallError{syscall, SockSetOptInt})
	}
	recordFD, recordLevel, recordOption, recordValue, errno, err := r.codec.DecodeSockSetOptInt(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
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
			r.handle(errors.Join(mismatch...))
		}
	}
	return errno
}

func (r *Replayer) SockLocalAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockLocalAddress, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return nil, ENOSYS
			}
			return se.SockLocalAddress(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockLocalAddress {
		r.handle(&UnexpectedSyscallError{syscall, SockLocalAddress})
	}
	recordFD, addr, errno, err := r.codec.DecodeSockLocalAddress(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{SockLocalAddress, "fd", fd, recordFD})
	}
	return addr, errno
}

func (r *Replayer) SockPeerAddress(ctx context.Context, fd FD) (SocketAddress, Errno) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		if s, ok := r.isEOF(SockPeerAddress, err); ok {
			se, ok := s.(SocketsExtension)
			if !ok {
				return nil, ENOSYS
			}
			return se.SockPeerAddress(ctx, fd)
		}
		r.handle(&ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != SockPeerAddress {
		r.handle(&UnexpectedSyscallError{syscall, SockPeerAddress})
	}
	recordFD, addr, errno, err := r.codec.DecodeSockPeerAddress(record.FunctionCall())
	if err != nil {
		r.handle(&DecodeError{record, err})
	}
	if r.strict && fd != recordFD {
		r.handle(&UnexpectedSyscallParamError{SockPeerAddress, "fd", fd, recordFD})
	}
	return addr, errno
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
	Recorded Syscall
	Observed Syscall
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
