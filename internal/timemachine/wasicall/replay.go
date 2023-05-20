package wasicall

import (
	"context"
	"fmt"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	. "github.com/stealthrocket/wasi-go"
)

// Replayer implements wasi.System by replaying system calls recorded in a log.
//
// There are a few special cases to consider:
//   - a system call may be requested after the log has been replayed fully. In
//     this case, the replayer will call the eof callback and then forward
//     subsequent calls to the wasi.System is provides
//   - an error may occur when reading a record from the log or decoding its
//     contents. In this case, the error callback is called with a ReadError or
//     DecodeError
//   - a system call that doesn't match the next record in the log may be
//     requested. In this case, the error callback is called with a
//     MismatchError
//   - if the ProcExit system call is called, the replayer will call the exit
//     callback rather than returning from the call
//
// The error and exit callbacks must not return normally. Rather, they should
// call panic(sys.NewExitError(...)) with an error code to halt execution of
// the WebAssembly module.
type Replayer struct {
	reader  *timemachine.LogRecordReader
	decoder Decoder

	eof   func() System
	error func(error)
	exit  func(ExitCode)

	eofSystem System
}

// NewReplayer creates a Replayer.
func NewReplayer(reader *timemachine.LogRecordReader, eof func() System, error func(error), exit func(ExitCode)) *Replayer {
	return &Replayer{
		reader: reader,
		eof:    eof,
		error:  error,
		exit:   exit,
	}
}

func (r *Replayer) isEOF() (System, bool) {
	if !r.reader.Next() {
		if r.eofSystem == nil {
			r.eofSystem = r.eof()
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
	if s, ok := r.isEOF(); ok {
		return s.ArgsGet(ctx)
	}
	record, err := r.reader.Record()
	if err != nil {
		r.handle(ReadError{err})
	}
	if Syscall(record.FunctionID()) != ArgsGet {
		r.handle(MismatchError{fmt.Errorf("expected syscall args_get (%d), got %d", ArgsGet, record.FunctionID())})
	}
	args, errno, err := r.decoder.ArgsGet(record.FunctionCall())
	if err != nil {
		r.handle(DecodeError{err})
	}
	return args, errno
}

// var _ wasi.System = (*Replayer)(nil)
// var _ wasi.SocketsExtension = (*Replayer)(nil)

type ReadError struct{ error }
type DecodeError struct{ error }
type MismatchError struct{ error }

func unreachable() {
	panic("unreachable")
}
