package wasicall

import (
	"fmt"
	"io"
	"time"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go"
)

// Reader reads recorded system calls.
//
// This is similar to Replay, but it allows the caller to drive the
// consumption of log records, and is required for offline consumption
// and analysis.

// NewReader creates a Reader.
func NewReader(records stream.Reader[timemachine.Record]) *Reader {
	r := &Reader{}
	r.records.Reset(records)
	return r
}

type Reader struct {
	records stream.Iterator[timemachine.Record]
	decoder Decoder
}

// ReadSyscall reads a recorded system call.
func (r *Reader) ReadSyscall() (time.Time, Syscall, error) {
	if !r.records.Next() {
		err := r.records.Err()
		if err == nil {
			err = io.EOF
		}
		return time.Time{}, nil, err
	}
	return r.decoder.Decode(r.records.Value())
}

// Decoder decodes syscalls from records.
type Decoder struct {
	codec Codec

	// Cache for decoded slices.
	args          []string
	iovecs        []wasi.IOVec
	subscriptions []wasi.Subscription
	events        []wasi.Event
	entries       []wasi.DirEntry
	addrinfo      []wasi.AddressInfo
}

// Decode a syscall from a record. Slices in the returned syscall point to
// internal cache buffers can are invalidated on the next Decode call.
func (r *Decoder) Decode(record timemachine.Record) (time.Time, Syscall, error) {
	// TODO: eliminate allocations by caching the Syscall
	//  instances on *Reader
	var syscall Syscall
	switch SyscallID(record.FunctionID) {
	case ArgsSizesGet:
		argCount, stringBytes, errno, err := r.codec.DecodeArgsSizesGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &ArgsSizesGetSyscall{argCount, stringBytes, errno}
	case ArgsGet:
		args, errno, err := r.codec.DecodeArgsGet(record.FunctionCall, r.args[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.args = args
		syscall = &ArgsGetSyscall{args, errno}
	case EnvironSizesGet:
		envCount, stringBytes, errno, err := r.codec.DecodeEnvironSizesGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &EnvironSizesGetSyscall{envCount, stringBytes, errno}
	case EnvironGet:
		env, errno, err := r.codec.DecodeEnvironGet(record.FunctionCall, r.args[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.args = env
		syscall = &EnvironGetSyscall{env, errno}
	case ClockResGet:
		clockID, timestamp, errno, err := r.codec.DecodeClockResGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &ClockResGetSyscall{clockID, timestamp, errno}
	case ClockTimeGet:
		clockID, precision, timestamp, errno, err := r.codec.DecodeClockTimeGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &ClockTimeGetSyscall{clockID, precision, timestamp, errno}
	case FDAdvise:
		fd, offset, length, advice, errno, err := r.codec.DecodeFDAdvise(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDAdviseSyscall{fd, offset, length, advice, errno}
	case FDAllocate:
		fd, offset, length, errno, err := r.codec.DecodeFDAllocate(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDAllocateSyscall{fd, offset, length, errno}
	case FDClose:
		fd, errno, err := r.codec.DecodeFDClose(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDCloseSyscall{fd, errno}
	case FDDataSync:
		fd, errno, err := r.codec.DecodeFDDataSync(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDDataSyncSyscall{fd, errno}
	case FDStatGet:
		fd, stat, errno, err := r.codec.DecodeFDStatGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDStatGetSyscall{fd, stat, errno}
	case FDStatSetFlags:
		fd, flags, errno, err := r.codec.DecodeFDStatSetFlags(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDStatSetFlagsSyscall{fd, flags, errno}
	case FDStatSetRights:
		fd, rightsBase, rightsInheriting, errno, err := r.codec.DecodeFDStatSetRights(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDStatSetRightsSyscall{fd, rightsBase, rightsInheriting, errno}
	case FDFileStatGet:
		fd, stat, errno, err := r.codec.DecodeFDFileStatGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDFileStatGetSyscall{fd, stat, errno}
	case FDFileStatSetSize:
		fd, size, errno, err := r.codec.DecodeFDFileStatSetSize(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDFileStatSetSizeSyscall{fd, size, errno}
	case FDFileStatSetTimes:
		fd, accessTime, modifyTime, flags, errno, err := r.codec.DecodeFDFileStatSetTimes(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDFileStatSetTimesSyscall{fd, accessTime, modifyTime, flags, errno}
	case FDPread:
		fd, iovecs, offset, size, errno, err := r.codec.DecodeFDPread(record.FunctionCall, r.iovecs[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.iovecs = iovecs
		syscall = &FDPreadSyscall{fd, iovecs, offset, size, errno}
	case FDPreStatGet:
		fd, stat, errno, err := r.codec.DecodeFDPreStatGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDPreStatGetSyscall{fd, stat, errno}
	case FDPreStatDirName:
		fd, name, errno, err := r.codec.DecodeFDPreStatDirName(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDPreStatDirNameSyscall{fd, name, errno}
	case FDPwrite:
		fd, iovecs, offset, size, errno, err := r.codec.DecodeFDPwrite(record.FunctionCall, r.iovecs[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.iovecs = iovecs
		syscall = &FDPwriteSyscall{fd, iovecs, offset, size, errno}
	case FDRead:
		fd, iovecs, size, errno, err := r.codec.DecodeFDRead(record.FunctionCall, r.iovecs[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.iovecs = iovecs
		syscall = &FDReadSyscall{fd, iovecs, size, errno}
	case FDReadDir:
		fd, entries, cookie, bufferSizeBytes, errno, err := r.codec.DecodeFDReadDir(record.FunctionCall, r.entries[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.entries = entries
		syscall = &FDReadDirSyscall{fd, entries, cookie, bufferSizeBytes, errno}
	case FDRenumber:
		from, to, errno, err := r.codec.DecodeFDRenumber(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDRenumberSyscall{from, to, errno}
	case FDSeek:
		fd, offset, whence, size, errno, err := r.codec.DecodeFDSeek(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDSeekSyscall{fd, offset, whence, size, errno}
	case FDSync:
		fd, errno, err := r.codec.DecodeFDSync(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDSyncSyscall{fd, errno}
	case FDTell:
		fd, size, errno, err := r.codec.DecodeFDTell(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &FDTellSyscall{fd, size, errno}
	case FDWrite:
		fd, iovecs, size, errno, err := r.codec.DecodeFDWrite(record.FunctionCall, r.iovecs[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.iovecs = iovecs
		syscall = &FDWriteSyscall{fd, iovecs, size, errno}
	case PathCreateDirectory:
		fd, path, errno, err := r.codec.DecodePathCreateDirectory(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathCreateDirectorySyscall{fd, path, errno}
	case PathFileStatGet:
		fd, lookupFlags, path, stat, errno, err := r.codec.DecodePathFileStatGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathFileStatGetSyscall{fd, lookupFlags, path, stat, errno}
	case PathFileStatSetTimes:
		fd, lookupFlags, path, accessTime, modifyTime, flags, errno, err := r.codec.DecodePathFileStatSetTimes(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathFileStatSetTimesSyscall{fd, lookupFlags, path, accessTime, modifyTime, flags, errno}
	case PathLink:
		oldFD, oldFlags, oldPath, newFD, newPath, errno, err := r.codec.DecodePathLink(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathLinkSyscall{oldFD, oldFlags, oldPath, newFD, newPath, errno}
	case PathOpen:
		fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno, err := r.codec.DecodePathOpen(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathOpenSyscall{fd, dirFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags, newfd, errno}
	case PathReadLink:
		fd, path, output, errno, err := r.codec.DecodePathReadLink(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathReadLinkSyscall{fd, path, output, errno}
	case PathRemoveDirectory:
		fd, path, errno, err := r.codec.DecodePathRemoveDirectory(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathRemoveDirectorySyscall{fd, path, errno}
	case PathRename:
		fd, oldPath, newFD, newPath, errno, err := r.codec.DecodePathRename(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathRenameSyscall{fd, oldPath, newFD, newPath, errno}
	case PathSymlink:
		oldPath, fd, newPath, errno, err := r.codec.DecodePathSymlink(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathSymlinkSyscall{oldPath, fd, newPath, errno}
	case PathUnlinkFile:
		fd, path, errno, err := r.codec.DecodePathUnlinkFile(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &PathUnlinkFileSyscall{fd, path, errno}
	case PollOneOff:
		subscriptions, events, errno, err := r.codec.DecodePollOneOff(record.FunctionCall, r.subscriptions[:0], r.events[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.subscriptions = subscriptions
		r.events = events
		syscall = &PollOneOffSyscall{subscriptions, events, errno}
	case ProcExit:
		exitCode, errno, err := r.codec.DecodeProcExit(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &ProcExitSyscall{exitCode, errno}
	case ProcRaise:
		signal, errno, err := r.codec.DecodeProcRaise(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &ProcRaiseSyscall{signal, errno}
	case SchedYield:
		errno, err := r.codec.DecodeSchedYield(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SchedYieldSyscall{errno}
	case RandomGet:
		b, errno, err := r.codec.DecodeRandomGet(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &RandomGetSyscall{b, errno}
	case SockAccept:
		fd, flags, newfd, peer, addr, errno, err := r.codec.DecodeSockAccept(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockAcceptSyscall{fd, flags, newfd, peer, addr, errno}
	case SockShutdown:
		fd, flags, errno, err := r.codec.DecodeSockShutdown(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockShutdownSyscall{fd, flags, errno}
	case SockRecv:
		fd, iovecs, iflags, size, oflags, errno, err := r.codec.DecodeSockRecv(record.FunctionCall, r.iovecs[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.iovecs = iovecs
		syscall = &SockRecvSyscall{fd, iovecs, iflags, size, oflags, errno}
	case SockSend:
		fd, iovecs, iflags, size, errno, err := r.codec.DecodeSockSend(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockSendSyscall{fd, iovecs, iflags, size, errno}
	case SockOpen:
		family, socketType, protocol, rightsBase, rightsInheriting, fd, errno, err := r.codec.DecodeSockOpen(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockOpenSyscall{family, socketType, protocol, rightsBase, rightsInheriting, fd, errno}
	case SockBind:
		fd, bind, addr, errno, err := r.codec.DecodeSockBind(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockBindSyscall{fd, bind, addr, errno}
	case SockConnect:
		fd, peer, addr, errno, err := r.codec.DecodeSockConnect(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockConnectSyscall{fd, peer, addr, errno}
	case SockListen:
		fd, backlog, errno, err := r.codec.DecodeSockListen(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockListenSyscall{fd, backlog, errno}
	case SockSendTo:
		fd, iovecs, iflags, addr, size, errno, err := r.codec.DecodeSockSendTo(record.FunctionCall, r.iovecs[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.iovecs = iovecs
		syscall = &SockSendToSyscall{fd, iovecs, iflags, addr, size, errno}
	case SockRecvFrom:
		fd, iovecs, iflags, size, oflags, addr, errno, err := r.codec.DecodeSockRecvFrom(record.FunctionCall, r.iovecs[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.iovecs = iovecs
		syscall = &SockRecvFromSyscall{fd, iovecs, iflags, size, oflags, addr, errno}
	case SockGetOpt:
		fd, level, option, value, errno, err := r.codec.DecodeSockGetOpt(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockGetOptSyscall{fd, level, option, value, errno}
	case SockSetOpt:
		fd, level, option, value, errno, err := r.codec.DecodeSockSetOpt(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockSetOptSyscall{fd, level, option, value, errno}
	case SockLocalAddress:
		fd, addr, errno, err := r.codec.DecodeSockLocalAddress(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockLocalAddressSyscall{fd, addr, errno}
	case SockRemoteAddress:
		fd, addr, errno, err := r.codec.DecodeSockRemoteAddress(record.FunctionCall)
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		syscall = &SockRemoteAddressSyscall{fd, addr, errno}
	case SockAddressInfo:
		name, service, hint, results, errno, err := r.codec.DecodeSockAddressInfo(record.FunctionCall, r.addrinfo[:0])
		if err != nil {
			return time.Time{}, nil, &DecodeError{record, err}
		}
		r.addrinfo = results
		syscall = &SockAddressInfoSyscall{name, service, hint, results, errno}
	default:
		return time.Time{}, nil, fmt.Errorf("unknown syscall %d", record.FunctionID)
	}
	return record.Time, syscall, nil
}
