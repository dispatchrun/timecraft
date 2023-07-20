package chaos

import (
	"context"

	"github.com/stealthrocket/wasi-go"
)

// Chunk wraps the base system to return one that chunks reads and writes to
// only partially use the I/O vectors. The intent is to exercise paths that
// handle successful but partial completion of I/O operations.
//
// This fault injector has a side effect of increasing latency of I/O operations
// because it breaks down the buffering layers that are usually implemented in
// applications, which may cause measurable slow downs if triggered too often.
//
// The filter does not apply to datagram sockets because truncating messages
// may break network protocols such as DNS, and the intent of this wrapper is
// not to inject irrecoverable errors.
func Chunk(base wasi.System) wasi.System {
	return &chunkSystem{System: base}
}

func canBeChunked(fileType wasi.FileType) bool {
	return fileType != wasi.SocketDGramType
}

type chunkSystem struct {
	wasi.System
	iovs [1]wasi.IOVec
}

func (s *chunkSystem) chunk(fileType wasi.FileType, iovs []wasi.IOVec) []wasi.IOVec {
	if canBeChunked(fileType) {
		for _, iov := range iovs {
			if len(iov) != 0 {
				s.iovs[0] = iov[:1]
				return s.iovs[:]
			}
		}
	}
	return iovs
}

func (s *chunkSystem) reset() {
	s.iovs[0] = nil
}

func (s *chunkSystem) FDPread(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	f, errno := s.System.FDStatGet(ctx, fd)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	iovs = s.chunk(f.FileType, iovs)
	defer s.reset()
	return s.System.FDPread(ctx, fd, iovs, offset)
}

func (s *chunkSystem) FDPwrite(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	f, errno := s.System.FDStatGet(ctx, fd)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	iovs = s.chunk(f.FileType, iovs)
	defer s.reset()
	return s.System.FDPwrite(ctx, fd, iovs, offset)
}

func (s *chunkSystem) FDRead(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	f, errno := s.System.FDStatGet(ctx, fd)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	iovs = s.chunk(f.FileType, iovs)
	defer s.reset()
	return s.System.FDRead(ctx, fd, iovs)
}

func (s *chunkSystem) FDWrite(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	f, errno := s.System.FDStatGet(ctx, fd)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	iovs = s.chunk(f.FileType, iovs)
	defer s.reset()
	return s.System.FDWrite(ctx, fd, iovs)
}

func (s *chunkSystem) SockRecv(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, iflags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	f, errno := s.System.FDStatGet(ctx, fd)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), wasi.ROFlags(0), errno
	}
	iovs = s.chunk(f.FileType, iovs)
	defer s.reset()
	return s.System.SockRecv(ctx, fd, iovs, iflags)
}

func (s *chunkSystem) SockSend(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, iflags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	f, errno := s.System.FDStatGet(ctx, fd)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	iovs = s.chunk(f.FileType, iovs)
	defer s.reset()
	return s.System.SockSend(ctx, fd, iovs, iflags)
}

func (s *chunkSystem) SockRecvFrom(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, iflags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	f, errno := s.System.FDStatGet(ctx, fd)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), wasi.ROFlags(0), nil, errno
	}
	iovs = s.chunk(f.FileType, iovs)
	defer s.reset()
	return s.System.SockRecvFrom(ctx, fd, iovs, iflags)
}

func (s *chunkSystem) SockSendTo(ctx context.Context, fd wasi.FD, iovs []wasi.IOVec, iflags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	f, errno := s.System.FDStatGet(ctx, fd)
	if errno != wasi.ESUCCESS {
		return ^wasi.Size(0), errno
	}
	iovs = s.chunk(f.FileType, iovs)
	defer s.reset()
	return s.System.SockSendTo(ctx, fd, iovs, iflags, addr)
}
