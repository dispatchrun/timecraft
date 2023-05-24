package wasicall

import (
	"fmt"
	"time"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go"
)

// Reader reads recorded system calls.
//
// This is similar to Replay, but it allows the caller to drive the
// consumption of log records, and is required for offline consumption
// and analysis.
type Reader struct {
	reader timemachine.RecordReader

	codec Codec

	// Cache for decoded slices.
	args          []string
	iovecs        []wasi.IOVec
	subscriptions []wasi.Subscription
	events        []wasi.Event
	entries       []wasi.DirEntry
}

// NewReader creates a Reader.
func NewReader(recordReader timemachine.RecordReader) *Reader {
	return &Reader{reader: recordReader}
}

// ReadSyscall reads a recorded system call.
func (r *Reader) ReadSyscall() (time.Time, Syscall, error) {
	record, err := r.reader.ReadRecord()
	if err != nil {
		return time.Time{}, nil, err
	}
	var syscall Syscall
	switch SyscallID(record.FunctionID()) {
	case ArgsSizesGet:
		argCount, stringBytes, errno, err := r.codec.DecodeArgsSizesGet(record.FunctionCall())
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &ArgsSizesGetSyscall{argCount, stringBytes, errno}
	case ArgsGet:
		args, errno, err := r.codec.DecodeArgsGet(record.FunctionCall(), r.args[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.args = args
		syscall = &ArgsGetSyscall{args, errno}
	case EnvironSizesGet:
		envCount, stringBytes, errno, err := r.codec.DecodeEnvironSizesGet(record.FunctionCall())
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &EnvironSizesGetSyscall{envCount, stringBytes, errno}
	case EnvironGet:
		env, errno, err := r.codec.DecodeEnvironGet(record.FunctionCall(), r.args[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.args = env
		syscall = &EnvironGetSyscall{env, errno}
	case ClockResGet:
	case ClockTimeGet:
	case FDAdvise:
	case FDAllocate:
	case FDClose:
	case FDDataSync:
	case FDStatGet:
	case FDStatSetFlags:
	case FDStatSetRights:
	case FDFileStatGet:
	case FDFileStatSetSize:
	case FDFileStatSetTimes:
	case FDPread:
	case FDPreStatGet:
	case FDPreStatDirName:
	case FDPwrite:
	case FDRead:
	case FDReadDir:
	case FDRenumber:
	case FDSeek:
	case FDSync:
	case FDTell:
	case FDWrite:
	case PathCreateDirectory:
	case PathFileStatGet:
	case PathFileStatSetTimes:
	case PathLink:
	case PathOpen:
	case PathReadLink:
	case PathRemoveDirectory:
	case PathRename:
	case PathSymlink:
	case PathUnlinkFile:
	case PollOneOff:
	case ProcExit:
	case ProcRaise:
	case SchedYield:
	case RandomGet:
	case SockAccept:
	case SockRecv:
	case SockSend:
	case SockShutdown:
	case SockOpen:
	case SockBind:
	case SockConnect:
	case SockListen:
	case SockSendTo:
	case SockRecvFrom:
	case SockGetOptInt:
	case SockSetOptInt:
	case SockLocalAddress:
	case SockPeerAddress:
	default:
		return time.Time{}, nil, fmt.Errorf("unknown syscall %d", record.FunctionID())
	}
	return record.Timestamp(), syscall, nil
}
