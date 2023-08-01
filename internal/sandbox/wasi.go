package sandbox

import (
	"context"
	"io"
	"io/fs"
	"math"
	"path"
	"path/filepath"
	"time"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/htls"
	"github.com/stealthrocket/wasi-go"
	wasisys "github.com/stealthrocket/wasi-go/systems/unix"
	sysunix "golang.org/x/sys/unix"
	"golang.org/x/time/rate"
)

// File is the interface implemented by all flavors of files that can be
// registered in a sandboxed System.
type anyFile interface {
	wasi.File[anyFile]

	// Returns the underlying file descriptor that this file is opened on.
	Fd() uintptr

	SockAccept(ctx context.Context, flags wasi.FDFlags) (anyFile, wasi.Errno)
	SockBind(ctx context.Context, addr wasi.SocketAddress) wasi.Errno
	SockConnect(ctx context.Context, peer wasi.SocketAddress) wasi.Errno
	SockListen(ctx context.Context, backlog int) wasi.Errno
	SockRecv(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno)
	SockSend(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno)
	SockSendTo(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno)
	SockRecvFrom(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno)
	SockGetOpt(ctx context.Context, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno)
	SockSetOpt(ctx context.Context, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno
	SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno)
	SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno)
	SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno
}

// unimplementedFileMethods declares all the methods of the anyFile interface
// that are not supported by implementations which are not files or directories.
type unimplementedFileMethods struct{}

func (unimplementedFileMethods) FDAdvise(ctx context.Context, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) FDDataSync(ctx context.Context) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) FDPread(ctx context.Context, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return 0, wasi.EBADF
}

func (unimplementedFileMethods) FDPwrite(ctx context.Context, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return 0, wasi.EBADF
}

func (unimplementedFileMethods) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	return nil, wasi.EBADF
}

func (unimplementedFileMethods) FDSync(ctx context.Context) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	return 0, wasi.EBADF
}

func (unimplementedFileMethods) PathCreateDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return wasi.FileStat{}, wasi.EBADF
}

func (unimplementedFileMethods) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsDefault, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	return nil, wasi.EBADF
}

func (unimplementedFileMethods) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	return 0, wasi.EBADF
}

func (unimplementedFileMethods) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) PathRename(ctx context.Context, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	return wasi.EBADF
}

func (unimplementedFileMethods) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	return wasi.EBADF
}

// unimplementedSocketMethods is useful to declare all file methods as not implemented by
// embedding the type.
//
// Only methods that are not valid to call or files and directories are declared.
type unimplementedSocketMethods struct{}

func (unimplementedSocketMethods) SockBind(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockConnect(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockListen(ctx context.Context, backlog int) wasi.Errno {
	return wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockAccept(ctx context.Context, flags wasi.FDFlags) (anyFile, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockRecv(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	return 0, 0, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockRecvFrom(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	return 0, 0, nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockSend(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockSendTo(ctx context.Context, iovs []wasi.IOVec, flags wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	return 0, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockGetOpt(ctx context.Context, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockSetOpt(ctx context.Context, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	return wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	return nil, wasi.ENOTSOCK
}

func (unimplementedSocketMethods) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	return wasi.ENOTSOCK
}

type wasiFile struct {
	unimplementedSocketMethods
	file File
}

func (f *wasiFile) Fd() uintptr {
	return f.file.Fd()
}

func (f *wasiFile) FDAdvise(ctx context.Context, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return wasi.ESUCCESS
}

func (f *wasiFile) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	return wasi.ESUCCESS
}

func (f *wasiFile) FDClose(ctx context.Context) wasi.Errno {
	return wasi.MakeErrno(f.file.Close())
}

func (f *wasiFile) FDDataSync(ctx context.Context) wasi.Errno {
	return wasi.MakeErrno(f.file.Datasync())
}

func (f *wasiFile) FDFileStatGet(ctx context.Context) (stat wasi.FileStat, errno wasi.Errno) {
	errno = wasi.ENOSYS
	return
}

func (f *wasiFile) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	sysFlags, err := f.file.Flags()
	if err != nil {
		return wasi.MakeErrno(err)
	}
	if flags.Has(wasi.Append) {
		sysFlags |= O_APPEND
	}
	if flags.Has(wasi.DSync) {
		sysFlags |= O_DSYNC
	}
	if flags.Has(wasi.RSync) {
		sysFlags |= O_RSYNC
	}
	if flags.Has(wasi.Sync) {
		sysFlags |= O_SYNC
	}
	return wasi.MakeErrno(f.file.SetFlags(sysFlags))
}

func (f *wasiFile) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	return wasi.MakeErrno(f.file.Truncate(int64(size)))
}

func (f *wasiFile) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	atime := makeFileTime(accessTime, flags.Has(wasi.AccessTime), flags.Has(wasi.AccessTimeNow))
	mtime := makeFileTime(modifyTime, flags.Has(wasi.ModifyTime), flags.Has(wasi.ModifyTimeNow))
	return wasi.MakeErrno(f.file.Chtimes("", atime, mtime, 0))
}

func (f *wasiFile) FDRead(ctx context.Context, iovs []wasi.IOVec) (size wasi.Size, errno wasi.Errno) {
	for _, iov := range iovs {
		n, err := f.file.Read(iov)
		size += wasi.Size(n)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return size, wasi.MakeErrno(err)
		}
	}
	return size, wasi.ESUCCESS
}

func (f *wasiFile) FDPread(ctx context.Context, iovs []wasi.IOVec, offset wasi.FileSize) (size wasi.Size, errno wasi.Errno) {
	for _, iov := range iovs {
		n, err := f.file.ReadAt(iov, int64(offset))
		size += wasi.Size(n)
		offset += wasi.FileSize(n)
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return size, wasi.MakeErrno(err)
		}
	}
	return size, wasi.ESUCCESS
}

func (f *wasiFile) FDWrite(ctx context.Context, iovs []wasi.IOVec) (size wasi.Size, errno wasi.Errno) {
	for _, iov := range iovs {
		n, err := f.file.Write(iov)
		size += wasi.Size(n)
		if err != nil {
			return size, wasi.MakeErrno(err)
		}
	}
	return size, wasi.ESUCCESS
}

func (f *wasiFile) FDPwrite(ctx context.Context, iovs []wasi.IOVec, offset wasi.FileSize) (size wasi.Size, errno wasi.Errno) {
	for _, iov := range iovs {
		n, err := f.file.WriteAt(iov, int64(offset))
		size += wasi.Size(n)
		offset += wasi.FileSize(n)
		if err != nil {
			return size, wasi.MakeErrno(err)
		}
	}
	return size, wasi.ESUCCESS
}

func (f *wasiFile) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	dirents, err := f.file.ReadDir(-1)
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return &wasiDir{dirents}, wasi.ESUCCESS
}

func (f *wasiFile) FDSync(ctx context.Context) wasi.Errno {
	return wasi.MakeErrno(f.file.Sync())
}

func (f *wasiFile) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	offset, err := f.file.Seek(int64(delta), int(whence))
	return wasi.FileSize(offset), wasi.MakeErrno(err)
}

func (f *wasiFile) PathCreateDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.MakeErrno(f.file.Mkdir(path, 0777))
}

func (f *wasiFile) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return wasi.FileStat{}, wasi.EBADF
}

func (f *wasiFile) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	// TODO: lookup flags
	atime := makeFileTime(accessTime, fstFlags.Has(wasi.AccessTime), fstFlags.Has(wasi.AccessTimeNow))
	mtime := makeFileTime(modifyTime, fstFlags.Has(wasi.ModifyTime), fstFlags.Has(wasi.ModifyTimeNow))
	return wasi.MakeErrno(f.file.Chtimes(path, atime, mtime, makeLookupFlags(lookupFlags)))
}

func (f *wasiFile) PathLink(ctx context.Context, lookupFlags wasi.LookupFlags, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*wasiFile)
	if !ok {
		return wasi.EXDEV
	}
	return wasi.MakeErrno(f.file.Link(oldPath, d.file, newPath, makeLookupFlags(lookupFlags)))
}

func (f *wasiFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsDefault, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	return nil, wasi.EBADF
}

func (f *wasiFile) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	n, err := f.file.Readlink(path, buffer)
	return n, wasi.MakeErrno(err)
}

func (f *wasiFile) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	return wasi.MakeErrno(f.file.Rmdir(path))
}

func (f *wasiFile) PathRename(ctx context.Context, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*wasiFile)
	if !ok {
		return wasi.EXDEV
	}
	return wasi.MakeErrno(f.file.Rename(oldPath, d.file, newPath))
}

func (f *wasiFile) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	return wasi.MakeErrno(f.file.Symlink(oldPath, newPath))
}

func (f *wasiFile) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	return wasi.MakeErrno(f.file.Unlink(path))
}

func makeFileTime(t wasi.Timestamp, set, now bool) time.Time {
	if set {
		if now {
			return time.Unix(0, _UTIME_NOW)
		}
		return t.Time()
	}
	return time.Unix(0, _UTIME_OMIT)
}

func makeLookupFlags(lookupFlags wasi.LookupFlags) (flags int) {
	if !lookupFlags.Has(wasi.SymlinkFollow) {
		flags |= AT_SYMLINK_NOFOLLOW
	}
	return flags
}

// TODO: this implementation is not very optimal because it causes a lot of heap
// allocations for the fs.DirEntry values and the conversion to wasi.DirEntry;
// we should replace it with something that does not require loading all the
// entries in memory, and avoids the allocation of intermediary bytes slices to
// hold the entry names.
type wasiDir struct {
	entries []fs.DirEntry
}

func (d *wasiDir) FDCloseDir(ctx context.Context) wasi.Errno {
	d.entries = nil
	return wasi.ESUCCESS
}

func (d *wasiDir) FDReadDir(ctx context.Context, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (n int, errno wasi.Errno) {
	for i := int(cookie); n < len(entries) && i < len(d.entries); {
		e := d.entries[i]

		entries[n].Next = wasi.DirCookie(i + 1)
		entries[n].Type = wasiFileType(e.Type())
		entries[n].Name = []byte(e.Name())

		bufferSizeBytes -= wasi.SizeOfDirent
		bufferSizeBytes -= len(entries[n].Name)

		if bufferSizeBytes <= 0 {
			break
		}
		i++
		n++
	}
	return n, wasi.ESUCCESS
}

func wasiFileType(mode fs.FileMode) wasi.FileType {
	switch mode.Type() {
	case 0:
		return wasi.RegularFileType
	case fs.ModeDevice:
		return wasi.BlockDeviceType
	case fs.ModeDevice | fs.ModeCharDevice:
		return wasi.CharacterDeviceType
	case fs.ModeDir:
		return wasi.DirectoryType
	case fs.ModeSymlink:
		return wasi.SymbolicLinkType
	case fs.ModeSocket:
		return wasi.SocketStreamType
	default:
		return wasi.UnknownType
	}
}

type wasiSocket struct {
	unimplementedFileMethods
	socket Socket
}

func (s *wasiSocket) Fd() uintptr {
	return s.socket.Fd()
}

func (s *wasiSocket) FDClose(ctx context.Context) wasi.Errno {
	return wasi.MakeErrno(s.socket.Close())
}

func (s *wasiSocket) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	n, _, errno := s.SockRecv(ctx, iovs, 0)
	return n, errno
}

func (s *wasiSocket) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.SockSendTo(ctx, iovs, 0, nil)
}

func (s *wasiSocket) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return wasi.MakeErrno(s.socket.SetNonBlock(flags.Has(wasi.NonBlock)))
}

func (s *wasiSocket) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	var stat wasi.FileStat
	switch s.socket.Type() {
	case STREAM:
		stat.FileType = wasi.SocketStreamType
	case DGRAM:
		stat.FileType = wasi.SocketDGramType
	}
	return stat, wasi.ESUCCESS
}

func (s *wasiSocket) SockBind(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.MakeErrno(s.socket.Bind(toNetworkSockaddr(addr)))
}

func (s *wasiSocket) SockConnect(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.MakeErrno(s.socket.Connect(toNetworkSockaddr(addr)))
}

func (s *wasiSocket) SockListen(ctx context.Context, backlog int) wasi.Errno {
	return wasi.MakeErrno(s.socket.Listen(backlog))
}

func (s *wasiSocket) SockAccept(ctx context.Context, flags wasi.FDFlags) (anyFile, wasi.Errno) {
	socket, _, err := s.socket.Accept()
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	if err := socket.SetNonBlock(flags.Has(wasi.NonBlock)); err != nil {
		socket.Close()
		return nil, wasi.MakeErrno(err)
	}
	return &wasiSocket{socket: socket}, wasi.ESUCCESS
}

func (s *wasiSocket) SockRecv(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	n, rflags, _, err := s.socket.RecvFrom(makeIOVecs(iovs), recvFlags(flags))
	return wasi.Size(n), wasiROFlags(rflags), wasi.MakeErrno(err)
}

func (s *wasiSocket) SockRecvFrom(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	n, rflags, addr, err := s.socket.RecvFrom(makeIOVecs(iovs), recvFlags(flags))
	return wasi.Size(n), wasiROFlags(rflags), toWasiSocketAddress(addr), wasi.MakeErrno(err)
}

func (s *wasiSocket) SockSend(ctx context.Context, iovs []wasi.IOVec, _ wasi.SIFlags) (wasi.Size, wasi.Errno) {
	n, err := s.socket.SendTo(makeIOVecs(iovs), nil, 0)
	return wasi.Size(n), wasi.MakeErrno(err)
}

func (s *wasiSocket) SockSendTo(ctx context.Context, iovs []wasi.IOVec, _ wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	n, err := s.socket.SendTo(makeIOVecs(iovs), toNetworkSockaddr(addr), 0)
	return wasi.Size(n), wasi.MakeErrno(err)
}

func (s *wasiSocket) SockGetOpt(ctx context.Context, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	switch option {
	case wasi.ReuseAddress:
		return nil, wasi.ENOTSUP

	case wasi.QuerySocketType:
		switch s.socket.Type() {
		case STREAM:
			return wasi.IntValue(wasi.StreamSocket), wasi.ESUCCESS
		case DGRAM:
			return wasi.IntValue(wasi.DatagramSocket), wasi.ESUCCESS
		default:
			return nil, wasi.ENOTSUP
		}

	case wasi.QuerySocketError:
		return wasi.IntValue(wasi.MakeErrno(s.socket.Error())), wasi.ESUCCESS

	case wasi.DontRoute:
		return nil, wasi.ENOTSUP

	case wasi.Broadcast:
		return nil, wasi.ENOTSUP

	case wasi.SendBufferSize:
		v, err := s.socket.SendBuffer()
		return wasi.IntValue(v), wasi.MakeErrno(err)

	case wasi.RecvBufferSize:
		v, err := s.socket.RecvBuffer()
		return wasi.IntValue(v), wasi.MakeErrno(err)

	case wasi.KeepAlive:
		return nil, wasi.ENOTSUP

	case wasi.OOBInline:
		return nil, wasi.ENOTSUP

	case wasi.RecvLowWatermark:
		return nil, wasi.ENOTSUP

	case wasi.QueryAcceptConnections:
		listen, err := s.socket.IsListening()
		return boolToIntValue(listen), wasi.MakeErrno(err)

	case wasi.TcpNoDelay:
		nodelay, err := s.socket.TCPNoDelay()
		return boolToIntValue(nodelay), wasi.MakeErrno(err)

	case wasi.Linger:
		return nil, wasi.ENOTSUP

	case wasi.RecvTimeout:
		t, err := s.socket.RecvTimeout()
		return durationToTimeValue(t), wasi.MakeErrno(err)

	case wasi.SendTimeout:
		t, err := s.socket.SendTimeout()
		return durationToTimeValue(t), wasi.MakeErrno(err)

	case wasi.BindToDevice:
		return nil, wasi.ENOTSUP

	default:
		return nil, wasi.EINVAL
	}
}

func boolToIntValue(v bool) wasi.IntValue {
	if v {
		return 1
	}
	return 0
}

func durationToTimeValue(v time.Duration) wasi.TimeValue {
	return wasi.TimeValue(int64(v))
}

func (s *wasiSocket) SockSetOpt(ctx context.Context, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	var (
		htlsServerName = wasi.MakeSocketOption(htls.Level, htls.ServerName)
	)

	var err error
	switch option {
	case htlsServerName:
		serverName, ok := value.(wasi.BytesValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetTLSServerName(string(serverName))
		}

	case wasi.ReuseAddress:
		return wasi.ENOPROTOOPT

	case wasi.QuerySocketType:
		return wasi.ENOPROTOOPT

	case wasi.QuerySocketError:
		return wasi.ENOPROTOOPT

	case wasi.DontRoute:
		return wasi.ENOPROTOOPT

	case wasi.Broadcast:
		return wasi.ENOPROTOOPT

	case wasi.RecvBufferSize:
		size, ok := value.(wasi.IntValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetRecvBuffer(int(size))
		}

	case wasi.SendBufferSize:
		size, ok := value.(wasi.IntValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetSendBuffer(int(size))
		}

	case wasi.KeepAlive:
		return wasi.ENOPROTOOPT

	case wasi.OOBInline:
		return wasi.ENOPROTOOPT

	case wasi.RecvLowWatermark:
		return wasi.ENOPROTOOPT

	case wasi.QueryAcceptConnections:
		return wasi.ENOPROTOOPT

	case wasi.TcpNoDelay:
		nodelay, ok := value.(wasi.IntValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetTCPNoDelay(nodelay != 0)
		}

	case wasi.Linger:
		return wasi.ENOPROTOOPT

	case wasi.RecvTimeout:
		timeout, ok := value.(wasi.TimeValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetRecvTimeout(time.Duration(timeout))
		}

	case wasi.SendTimeout:
		timeout, ok := value.(wasi.TimeValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetSendTimeout(time.Duration(timeout))
		}

	case wasi.BindToDevice:
		return wasi.ENOPROTOOPT

	default:
		return wasi.EINVAL
	}
	return wasi.MakeErrno(err)
}

func (s *wasiSocket) SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	name, err := s.socket.Name()
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return toWasiSocketAddress(name), wasi.ESUCCESS
}

func (s *wasiSocket) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	peer, err := s.socket.Peer()
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return toWasiSocketAddress(peer), wasi.ESUCCESS
}

func (s *wasiSocket) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	var shut int
	if flags.Has(wasi.ShutdownRD) {
		shut |= SHUTRD
	}
	if flags.Has(wasi.ShutdownWR) {
		shut |= SHUTWR
	}
	return wasi.MakeErrno(s.socket.Shutdown(shut))
}

func toNetworkSockaddr(addr wasi.SocketAddress) Sockaddr {
	switch a := addr.(type) {
	case *wasi.Inet4Address:
		return &SockaddrInet4{
			Port: a.Port,
			Addr: a.Addr,
		}
	case *wasi.Inet6Address:
		return &SockaddrInet6{
			Port: a.Port,
			Addr: a.Addr,
		}
	case *wasi.UnixAddress:
		return &SockaddrUnix{
			Name: a.Name,
		}
	default:
		return nil
	}
}

func toWasiSocketAddress(sa Sockaddr) wasi.SocketAddress {
	switch t := sa.(type) {
	case *SockaddrInet4:
		return &wasi.Inet4Address{
			Addr: t.Addr,
			Port: t.Port,
		}
	case *SockaddrInet6:
		return &wasi.Inet6Address{
			Addr: t.Addr,
			Port: t.Port,
		}
	case *SockaddrUnix:
		name := t.Name
		if len(name) == 0 {
			// For consistency across platforms, replace empty unix socket
			// addresses with @. On Linux, addresses where the first byte is
			// a null byte are considered abstract unix sockets, and the first
			// byte is replaced with @.
			name = "@"
		}
		return &wasi.UnixAddress{
			Name: name,
		}
	default:
		return nil
	}
}

func makeIOVecs(iovs []wasi.IOVec) [][]byte {
	return *(*[][]byte)(unsafe.Pointer(&iovs))
}

func recvFlags(riflags wasi.RIFlags) (flags int) {
	if riflags.Has(wasi.RecvPeek) {
		flags |= PEEK
	}
	if riflags.Has(wasi.RecvWaitAll) {
		flags |= WAITALL
	}
	return flags
}

func wasiROFlags(rflags int) (roflags wasi.ROFlags) {
	if (rflags & TRUNC) != 0 {
		roflags |= wasi.RecvDataTruncated
	}
	return roflags
}

// wasiFS is an interface used to represent a sandbox file system.
//
// The interface has a single method allowing the sandbox to open the root
// directory.
type wasiFS interface {
	PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno)
}

// ErrFS returns a FS value which always returns the given error.
func ErrFS(errno wasi.Errno) wasiFS { return errFS(errno) }

type errFS wasi.Errno

func (err errFS) PathOpen(context.Context, wasi.LookupFlags, string, wasi.OpenFlags, wasi.Rights, wasi.Rights, wasi.FDFlags) (anyFile, wasi.Errno) {
	return nil, wasi.Errno(err)
}

// PathFS returns a wasiFS value which opens files from the given file system path.
//
// The path is resolved to an absolute path in order to guarantee that the FS is
// not dependent on the current working directory.
func PathFS(path string) wasiFS {
	path, err := filepath.Abs(path)
	if err != nil {
		return ErrFS(wasi.MakeErrno(err))
	}
	return wasiDirFS(filepath.ToSlash(path))
}

type wasiDirFS string

func (dir wasiDirFS) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, filePath string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	filePath = path.Join(string(dir), filePath)
	f, errno := wasisys.FD(sysunix.AT_FDCWD).PathOpen(ctx, lookupFlags, filePath, openFlags, rightsBase, rightsInheriting, fdFlags)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &wasiDirFile{fd: f}, wasi.ESUCCESS
}

type wasiDirFile struct {
	unimplementedSocketMethods
	fd wasisys.FD
}

func (f *wasiDirFile) Fd() uintptr {
	return uintptr(f.fd)
}

func (f *wasiDirFile) FDClose(ctx context.Context) wasi.Errno {
	return f.fd.FDClose(ctx)
}

func (f *wasiDirFile) FDAdvise(ctx context.Context, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	return f.fd.FDAdvise(ctx, offset, length, advice)
}

func (f *wasiDirFile) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	return f.fd.FDAllocate(ctx, offset, length)
}

func (f *wasiDirFile) FDDataSync(ctx context.Context) wasi.Errno {
	return f.fd.FDDataSync(ctx)
}

func (f *wasiDirFile) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return f.fd.FDStatSetFlags(ctx, flags)
}

func (f *wasiDirFile) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	return f.fd.FDFileStatGet(ctx)
}

func (f *wasiDirFile) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	return f.fd.FDFileStatSetSize(ctx, size)
}

func (f *wasiDirFile) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	return f.fd.FDFileStatSetTimes(ctx, accessTime, modifyTime, flags)
}

func (f *wasiDirFile) FDPread(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return f.fd.FDPread(ctx, iovecs, offset)
}

func (f *wasiDirFile) FDPwrite(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	return f.fd.FDPwrite(ctx, iovecs, offset)
}

func (f *wasiDirFile) FDRead(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return f.fd.FDRead(ctx, iovecs)
}

func (f *wasiDirFile) FDWrite(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return f.fd.FDWrite(ctx, iovecs)
}

func (f *wasiDirFile) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	return f.fd.FDOpenDir(ctx)
}

func (f *wasiDirFile) FDSync(ctx context.Context) wasi.Errno {
	return f.fd.FDSync(ctx)
}

func (f *wasiDirFile) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	return f.fd.FDSeek(ctx, delta, whence)
}

func (f *wasiDirFile) PathCreateDirectory(ctx context.Context, path string) wasi.Errno {
	return f.fd.PathCreateDirectory(ctx, path)
}

func (f *wasiDirFile) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	return f.fd.PathFileStatGet(ctx, flags, path)
}

func (f *wasiDirFile) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	return f.fd.PathFileStatSetTimes(ctx, lookupFlags, path, accessTime, modifyTime, fstFlags)
}

func (f *wasiDirFile) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*wasiDirFile)
	if !ok {
		return wasi.EXDEV
	}
	return f.fd.PathLink(ctx, flags, oldPath, d.fd, newPath)
}

func (f *wasiDirFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	fd, errno := f.fd.PathOpen(ctx, lookupFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &wasiDirFile{fd: fd}, wasi.ESUCCESS
}

func (f *wasiDirFile) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	return f.fd.PathReadLink(ctx, path, buffer)
}

func (f *wasiDirFile) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	return f.fd.PathRemoveDirectory(ctx, path)
}

func (f *wasiDirFile) PathRename(ctx context.Context, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*wasiDirFile)
	if !ok {
		return wasi.EXDEV
	}
	return f.fd.PathRename(ctx, oldPath, d.fd, newPath)
}

func (f *wasiDirFile) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	return f.fd.PathSymlink(ctx, oldPath, newPath)
}

func (f *wasiDirFile) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	return f.fd.PathUnlinkFile(ctx, path)
}

// ThrottleFS wraps the file system passed as argument to apply the rate limits
// r and w on read and write operations.
//
// The limits apply to all access to the underlying file system which may result
// in I/O operations.
//
// Passing a nil rate limiter to r or w disables rate limiting on the
// corresponding I/O operations.
func ThrottleFS(f wasiFS, r, w *rate.Limiter) wasiFS {
	return &throttleFS{base: f, rlim: r, wlim: w}
}

type throttleFS struct {
	base wasiFS
	rlim *rate.Limiter
	wlim *rate.Limiter
}

func (fsys *throttleFS) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	f, errno := fsys.base.PathOpen(ctx, lookupFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if canThrottle(errno) {
		fsys.throttlePathRead(ctx, path, 1)
	}
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &throttleFile{base: f, fsys: fsys}, wasi.ESUCCESS
}

func (fsys *throttleFS) throttlePathRead(ctx context.Context, path string, n int64) {
	fsys.throttleRead(ctx, int64(len(path))+alignIOPageSize(n))
}

func (fsys *throttleFS) throttlePathWrite(ctx context.Context, path string, n int64) {
	fsys.throttleRead(ctx, int64(len(path)))
	fsys.throttleWrite(ctx, n)
}

func (fsys *throttleFS) throttleRead(ctx context.Context, n int64) {
	throttle(ctx, fsys.rlim, alignIOPageSize(n))
}

func (fsys *throttleFS) throttleWrite(ctx context.Context, n int64) {
	throttle(ctx, fsys.wlim, alignIOPageSize(n))
}

func alignIOPageSize(n int64) int64 {
	const pageSize = 4096
	return ((n + pageSize - 1) / pageSize) * pageSize
}

func throttle(ctx context.Context, l *rate.Limiter, n int64) {
	if l == nil {
		return
	}
	for n > 0 {
		waitN := n
		if waitN > math.MaxInt32 {
			waitN = math.MaxInt32
		}
		if err := l.WaitN(ctx, int(waitN)); err != nil {
			panic(err)
		}
		n -= waitN
	}
}

type throttleFile struct {
	unimplementedSocketMethods
	base anyFile
	fsys *throttleFS
}

func (f *throttleFile) Fd() uintptr {
	return f.base.Fd()
}

func (f *throttleFile) FDClose(ctx context.Context) wasi.Errno {
	return f.base.FDClose(ctx)
}

func (f *throttleFile) FDAdvise(ctx context.Context, offset, length wasi.FileSize, advice wasi.Advice) wasi.Errno {
	errno := f.base.FDAdvise(ctx, offset, length, advice)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, int64(length))
	}
	return errno
}

func (f *throttleFile) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	errno := f.base.FDAllocate(ctx, offset, length)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, int64(length))
	}
	return errno
}

func (f *throttleFile) FDDataSync(ctx context.Context) wasi.Errno {
	s, errno := f.base.FDFileStatGet(ctx)
	if errno != wasi.ESUCCESS {
		return errno
	}
	errno = f.base.FDDataSync(ctx)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, int64(s.Size))
	}
	return errno
}

func (f *throttleFile) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	return f.base.FDStatSetFlags(ctx, flags)
}

func (f *throttleFile) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	stat, errno := f.base.FDFileStatGet(ctx)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, 1)
	}
	return stat, errno
}

func (f *throttleFile) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	errno := f.base.FDFileStatSetSize(ctx, size)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, int64(size))
	}
	return errno
}

func (f *throttleFile) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	errno := f.base.FDFileStatSetTimes(ctx, accessTime, modifyTime, flags)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, 1)
	}
	return errno
}

func (f *throttleFile) FDPread(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	n, errno := f.base.FDPread(ctx, iovecs, offset)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, sizeToInt64(n))
	}
	return n, errno
}

func (f *throttleFile) FDPwrite(ctx context.Context, iovecs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	n, errno := f.base.FDPwrite(ctx, iovecs, offset)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, sizeToInt64(n))
	}
	return n, errno
}

func (f *throttleFile) FDRead(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	n, errno := f.base.FDRead(ctx, iovecs)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, sizeToInt64(n))
	}
	return n, errno
}

func (f *throttleFile) FDWrite(ctx context.Context, iovecs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	n, errno := f.base.FDWrite(ctx, iovecs)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, sizeToInt64(n))
	}
	return n, errno
}

func (f *throttleFile) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	d, errno := f.base.FDOpenDir(ctx)
	if canThrottle(errno) {
		f.fsys.throttleRead(ctx, 1)
	}
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &throttleDir{base: d, fsys: f.fsys}, wasi.ESUCCESS
}

func (f *throttleFile) FDSync(ctx context.Context) wasi.Errno {
	s, errno := f.base.FDFileStatGet(ctx)
	if errno != wasi.ESUCCESS {
		return errno
	}
	errno = f.base.FDSync(ctx)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, int64(s.Size))
	}
	return errno
}

func (f *throttleFile) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (wasi.FileSize, wasi.Errno) {
	offset, errno := f.base.FDSeek(ctx, delta, whence)
	if canThrottle(errno) {
		f.fsys.throttleWrite(ctx, 1)
	}
	return offset, errno
}

func (f *throttleFile) PathCreateDirectory(ctx context.Context, path string) wasi.Errno {
	errno := f.base.PathCreateDirectory(ctx, path)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, path, 1)
	}
	return errno
}

func (f *throttleFile) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	stat, errno := f.base.PathFileStatGet(ctx, flags, path)
	if canThrottle(errno) {
		f.fsys.throttlePathRead(ctx, path, 1)
	}
	return stat, errno
}

func (f *throttleFile) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	errno := f.base.PathFileStatSetTimes(ctx, lookupFlags, path, accessTime, modifyTime, fstFlags)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, path, 1)
	}
	return errno
}

func (f *throttleFile) PathLink(ctx context.Context, flags wasi.LookupFlags, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*throttleFile)
	if !ok {
		return wasi.EXDEV
	}
	errno := f.base.PathLink(ctx, flags, oldPath, d.base, newPath)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, oldPath, int64(len(newPath)))
	}
	return errno
}

func (f *throttleFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	newFile, errno := f.base.PathOpen(ctx, lookupFlags, path, openFlags, rightsBase, rightsInheriting, fdFlags)
	if canThrottle(errno) {
		f.fsys.throttlePathRead(ctx, path, 1)
	}
	if errno != wasi.ESUCCESS {
		return nil, errno
	}
	return &throttleFile{base: newFile, fsys: f.fsys}, wasi.ESUCCESS
}

func (f *throttleFile) PathReadLink(ctx context.Context, path string, buffer []byte) (int, wasi.Errno) {
	n, errno := f.base.PathReadLink(ctx, path, buffer)
	if canThrottle(errno) {
		f.fsys.throttlePathRead(ctx, path, int64(n))
	}
	return n, errno
}

func (f *throttleFile) PathRemoveDirectory(ctx context.Context, path string) wasi.Errno {
	errno := f.base.PathRemoveDirectory(ctx, path)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, path, 1)
	}
	return errno
}

func (f *throttleFile) PathRename(ctx context.Context, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*throttleFile)
	if !ok {
		return wasi.EXDEV
	}
	errno := f.base.PathRename(ctx, oldPath, d.base, newPath)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, oldPath, int64(len(newPath)))
	}
	return errno
}

func (f *throttleFile) PathSymlink(ctx context.Context, oldPath string, newPath string) wasi.Errno {
	errno := f.base.PathSymlink(ctx, oldPath, newPath)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, oldPath, int64(len(newPath)))
	}
	return errno
}

func (f *throttleFile) PathUnlinkFile(ctx context.Context, path string) wasi.Errno {
	errno := f.base.PathUnlinkFile(ctx, path)
	if canThrottle(errno) {
		f.fsys.throttlePathWrite(ctx, path, 1)
	}
	return errno
}

// canThrottle returns true if the given errno code indicates that throttling
// can be applied to the operation that it was returned from.
//
// Throttling is always applied after performing the operation because we cannot
// know in advance whether the arguments passed to the method are valid; we have
// to first make the call and determine after the fact if the error returned by
// the method indicates that the operation was aborted due to having invalid
// arguments, or it was attempted and we need to take the I/O operation cost
// into account.
//
// The list of errors here may not be exhaustive; future maintainers may choose
// to add more. Keep in mind that we are better off apply throttling in excess
// than missing conditions where it should be applied because malcious guests
// could take advantage of error conditions that caused I/O utilization but were
// not accounted for.
func canThrottle(errno wasi.Errno) bool {
	return errno != wasi.EBADF && errno != wasi.EINVAL && errno != wasi.ENOTCAPABLE
}

func sizeToInt64(size wasi.Size) int64 {
	return int64(int32(size)) // for sign extension
}

type throttleDir struct {
	base wasi.Dir
	fsys *throttleFS
}

func (d *throttleDir) FDReadDir(ctx context.Context, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	n, errno := d.base.FDReadDir(ctx, entries, cookie, bufferSizeBytes)
	if canThrottle(errno) {
		size := int64(0)
		for _, entry := range entries[:n] {
			size += wasi.SizeOfDirent
			size += int64(len(entry.Name))
		}
		d.fsys.throttleRead(ctx, size)
	}
	return n, errno
}

func (d *throttleDir) FDCloseDir(ctx context.Context) wasi.Errno {
	return d.base.FDCloseDir(ctx)
}
