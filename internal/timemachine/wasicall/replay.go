package wasicall

import (
	"context"
	"errors"
	"fmt"

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
// The error and exit callbacks must not return normally. Rather, they should
// call panic(sys.NewExitError(...)) with an error code to halt execution of
// the WebAssembly module.
type Replayer struct {
	reader  *timemachine.LogRecordReader
	decoder Decoder

	eof   func(Syscall) System
	error func(error)
	exit  func(ExitCode)

	validateParams bool

	eofSystem System
}

// var _ System = (*Replayer)(nil)
// var _ SocketsExtension = (*Replayer)(nil)

// NewReplayer creates a Replayer.
func NewReplayer(reader *timemachine.LogRecordReader, eof func(Syscall) System, error func(error), exit func(ExitCode)) *Replayer {
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

func (r *Replayer) isEOF(s Syscall) (System, bool) {
	if !r.reader.Next() {
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
	if s, ok := r.isEOF(ArgsGet); ok {
		return s.ArgsGet(ctx)
	}
	record, err := r.reader.Record()
	if err != nil {
		r.handle(ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != ArgsGet {
		r.handle(UnexpectedSyscallError{syscall, ArgsGet})
	}
	args, errno, err := r.decoder.ArgsGet(record.FunctionCall())
	if err != nil {
		r.handle(DecodeError{err})
	}
	return args, errno
}

func (r *Replayer) EnvironGet(ctx context.Context) ([]string, Errno) {
	if s, ok := r.isEOF(EnvironGet); ok {
		return s.EnvironGet(ctx)
	}
	record, err := r.reader.Record()
	if err != nil {
		r.handle(ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != EnvironGet {
		r.handle(UnexpectedSyscallError{syscall, EnvironGet})
	}
	env, errno, err := r.decoder.EnvironGet(record.FunctionCall())
	if err != nil {
		r.handle(DecodeError{err})
	}
	return env, errno
}

func (r *Replayer) ClockResGet(ctx context.Context, id ClockID) (Timestamp, Errno) {
	if s, ok := r.isEOF(ClockResGet); ok {
		return s.ClockResGet(ctx, id)
	}
	record, err := r.reader.Record()
	if err != nil {
		r.handle(ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != ClockResGet {
		r.handle(UnexpectedSyscallError{syscall, ClockResGet})
	}
	recordID, timestamp, errno, err := r.decoder.ClockResGet(record.FunctionCall())
	if err != nil {
		r.handle(DecodeError{err})
	}
	if r.validateParams {
		if recordID != id {
			r.handle(UnexpectedSyscallParamError{fmt.Errorf("unexpected ClockResGet.ID, got %v expect %v", id, recordID)})
		}
	}
	return timestamp, errno
}

func (r *Replayer) ClockTimeGet(ctx context.Context, id ClockID, precision Timestamp) (Timestamp, Errno) {
	if s, ok := r.isEOF(ClockTimeGet); ok {
		return s.ClockTimeGet(ctx, id, precision)
	}
	record, err := r.reader.Record()
	if err != nil {
		r.handle(ReadError{err})
	}
	if syscall := Syscall(record.FunctionID()); syscall != ClockTimeGet {
		r.handle(UnexpectedSyscallError{syscall, ClockTimeGet})
	}
	recordID, recordPrecision, timestamp, errno, err := r.decoder.ClockTimeGet(record.FunctionCall())
	if err != nil {
		r.handle(DecodeError{err})
	}
	if r.validateParams {
		var mismatch []error
		if recordID != id {
			mismatch = append(mismatch, UnexpectedSyscallParamError{fmt.Errorf("unexpected ClockTimeGet.ID, got %v expect %v", id, recordID)})
		}
		if recordPrecision != precision {
			mismatch = append(mismatch, UnexpectedSyscallParamError{fmt.Errorf("unexpected ClockTimeGet.Precision, got %v expect %v", precision, recordPrecision)})
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
type DecodeError struct{ error }

type UnexpectedSyscallParamError struct{ error }

type UnexpectedSyscallError struct{ actual, expect Syscall }

func (e UnexpectedSyscallError) Error() string {
	return fmt.Sprintf("expected syscall %s (%d) but got %s (%d)", e.expect, int(e.expect), e.actual, int(e.actual))
}

func unreachable() {
	panic("unreachable")
}
