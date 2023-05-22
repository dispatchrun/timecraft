package wasicall

import (
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
	codec  Codec

	eof   func(Syscall) System
	error func(error)
	exit  func(ExitCode)

	validateParams bool

	eofSystem System
}

// var _ System = (*Replayer)(nil)
// var _ SocketsExtension = (*Replayer)(nil)

// NewReplayer creates a Replayer.
func NewReplayer(reader timemachine.RecordReader, eof func(Syscall) System, error func(error), exit func(ExitCode)) *Replayer {
	return &Replayer{
		reader:         reader,
		eof:            eof,
		error:          error,
		exit:           exit,
		validateParams: true,
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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
	if r.validateParams {
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

func unreachable() {
	panic("unreachable")
}
