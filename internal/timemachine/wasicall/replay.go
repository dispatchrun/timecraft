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
