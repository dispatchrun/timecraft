package sandbox

import (
	"bytes"
	"context"
	"io/fs"
	"math"
	"time"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/htls"
	"github.com/stealthrocket/wasi-go"
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
	// TODO: should we implement this operation?
	return wasi.ESUCCESS
}

func (f *wasiFile) FDAllocate(ctx context.Context, offset, length wasi.FileSize) wasi.Errno {
	return wasi.MakeErrno(f.file.Allocate(int64(offset), int64(length)))
}

func (f *wasiFile) FDClose(ctx context.Context) wasi.Errno {
	return wasi.MakeErrno(f.file.Close())
}

func (f *wasiFile) FDDataSync(ctx context.Context) wasi.Errno {
	return wasi.MakeErrno(f.file.Datasync())
}

func (f *wasiFile) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	info, err := f.file.Stat("", 0)
	return wasiFileStat(info), wasi.MakeErrno(err)
}

func (f *wasiFile) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	sysFlags, err := f.file.Flags()
	if err != nil {
		return wasi.MakeErrno(err)
	}
	if flags.Has(wasi.Append) {
		sysFlags |= O_APPEND
	} else {
		sysFlags &= ^O_APPEND
	}
	if flags.Has(wasi.NonBlock) {
		sysFlags |= O_NONBLOCK
	} else {
		sysFlags &= ^O_NONBLOCK
	}
	return wasi.MakeErrno(f.file.SetFlags(sysFlags))
}

func (f *wasiFile) FDFileStatSetSize(ctx context.Context, size wasi.FileSize) wasi.Errno {
	return wasi.MakeErrno(f.file.Truncate(int64(size)))
}

func (f *wasiFile) FDFileStatSetTimes(ctx context.Context, accessTime, modifyTime wasi.Timestamp, flags wasi.FSTFlags) wasi.Errno {
	times := [2]Timespec{
		0: makeTimespec(accessTime, flags.Has(wasi.AccessTime), flags.Has(wasi.AccessTimeNow)),
		1: makeTimespec(modifyTime, flags.Has(wasi.ModifyTime), flags.Has(wasi.ModifyTimeNow)),
	}
	return wasi.MakeErrno(f.file.Chtimes("", times, 0))
}

func (f *wasiFile) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	n, err := f.file.Readv(makeIOVecs(iovs))
	return wasi.Size(n), wasi.MakeErrno(err)
}

func (f *wasiFile) FDPread(ctx context.Context, iovs []wasi.IOVec, offset wasi.FileSize) (wasi.Size, wasi.Errno) {
	n, err := f.file.Preadv(makeIOVecs(iovs), int64(offset))
	return wasi.Size(n), wasi.MakeErrno(err)
}

func (f *wasiFile) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	n, err := f.file.Writev(makeIOVecs(iovs))
	return wasi.Size(n), wasi.MakeErrno(err)
}

func (f *wasiFile) FDPwrite(ctx context.Context, iovs []wasi.IOVec, offset wasi.FileSize) (size wasi.Size, errno wasi.Errno) {
	n, err := f.file.Pwritev(makeIOVecs(iovs), int64(offset))
	return wasi.Size(n), wasi.MakeErrno(err)
}

func (f *wasiFile) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	if _, err := f.file.Seek(0, SEEK_SET); err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return &wasiDir{file: f.file}, wasi.ESUCCESS
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
	info, err := f.file.Stat(path, makeLookupFlags(flags))
	return wasiFileStat(info), wasi.MakeErrno(err)
}

func (f *wasiFile) PathFileStatSetTimes(ctx context.Context, lookupFlags wasi.LookupFlags, path string, accessTime, modifyTime wasi.Timestamp, fstFlags wasi.FSTFlags) wasi.Errno {
	times := [2]Timespec{
		0: makeTimespec(accessTime, fstFlags.Has(wasi.AccessTime), fstFlags.Has(wasi.AccessTimeNow)),
		1: makeTimespec(modifyTime, fstFlags.Has(wasi.ModifyTime), fstFlags.Has(wasi.ModifyTimeNow)),
	}
	return wasi.MakeErrno(f.file.Chtimes(path, times, makeLookupFlags(lookupFlags)))
}

func (f *wasiFile) PathLink(ctx context.Context, lookupFlags wasi.LookupFlags, oldPath string, newDir anyFile, newPath string) wasi.Errno {
	d, ok := newDir.(*wasiFile)
	if !ok {
		return wasi.EXDEV
	}
	return wasi.MakeErrno(f.file.Link(oldPath, d.file, newPath, makeLookupFlags(lookupFlags)))
}

func (f *wasiFile) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	flags := 0

	if openFlags.Has(wasi.OpenDirectory) {
		flags |= O_DIRECTORY
		rightsBase &= wasi.DirectoryRights
	}
	if openFlags.Has(wasi.OpenCreate) {
		flags |= O_CREAT
	}
	if openFlags.Has(wasi.OpenExclusive) {
		flags |= O_EXCL
	}
	if openFlags.Has(wasi.OpenTruncate) {
		flags |= O_TRUNC
	}
	if fdFlags.Has(wasi.Append) {
		flags |= O_APPEND
	}
	if fdFlags.Has(wasi.DSync) {
		flags |= O_DSYNC
	}
	if fdFlags.Has(wasi.Sync) {
		flags |= O_SYNC
	}
	if fdFlags.Has(wasi.RSync) {
		flags |= O_RSYNC
	}
	if fdFlags.Has(wasi.NonBlock) {
		flags |= O_NONBLOCK
	}
	if !lookupFlags.Has(wasi.SymlinkFollow) {
		flags |= O_NOFOLLOW
	}

	switch {
	case openFlags.Has(wasi.OpenDirectory):
		flags |= O_RDONLY
	case rightsBase.Has(wasi.FDReadRight) && rightsBase.Has(wasi.FDWriteRight):
		flags |= O_RDWR
	case rightsBase.Has(wasi.FDReadRight):
		flags |= O_RDONLY
	case rightsBase.Has(wasi.FDWriteRight):
		flags |= O_WRONLY
	default:
		flags |= O_RDONLY
	}

	mode := fs.FileMode(0644)
	if (flags & O_DIRECTORY) != 0 {
		mode = 0
	}

	f2, err := f.file.Open(path, flags, mode)
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return &wasiFile{file: f2}, wasi.ESUCCESS
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

func makeTimespec(t wasi.Timestamp, set, now bool) Timespec {
	if !set {
		return Timespec{Nsec: _UTIME_OMIT}
	}
	if now {
		return Timespec{Nsec: _UTIME_NOW}
	}
	return nsecToTimespec(int64(t))
}

func makeLookupFlags(lookupFlags wasi.LookupFlags) (flags int) {
	if !lookupFlags.Has(wasi.SymlinkFollow) {
		flags |= AT_SYMLINK_NOFOLLOW
	}
	return flags
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

func wasiFileStat(info FileInfo) wasi.FileStat {
	return wasi.FileStat{
		FileType:   wasiFileType(info.Mode),
		Device:     wasi.Device(info.Dev),
		INode:      wasi.INode(info.Ino),
		NLink:      wasi.LinkCount(info.Nlink),
		Size:       wasi.FileSize(info.Size),
		AccessTime: wasi.Timestamp(info.Atime.Nano()),
		ModifyTime: wasi.Timestamp(info.Mtime.Nano()),
		ChangeTime: wasi.Timestamp(info.Ctime.Nano()),
	}
}

type wasiDir struct {
	buffer *[dirbufsize]byte
	offset int
	length int
	file   File
	cookie wasi.DirCookie
}

func (d *wasiDir) FDCloseDir(ctx context.Context) wasi.Errno {
	d.file = nil
	return wasi.ESUCCESS
}

func (d *wasiDir) FDReadDir(ctx context.Context, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (int, wasi.Errno) {
	if d.file == nil {
		return 0, wasi.ESUCCESS // EOF
	}
	if d.buffer == nil {
		d.buffer = new([dirbufsize]byte)
	}

	if cookie < d.cookie {
		if _, err := d.file.Seek(0, SEEK_SET); err != nil {
			return 0, wasi.MakeErrno(err)
		}
		d.offset = 0
		d.length = 0
		d.cookie = 0
	}

	numEntries := 0
	for {
		if numEntries == len(entries) {
			return numEntries, wasi.ESUCCESS
		}

		if (d.length - d.offset) < sizeOfDirent {
			if numEntries > 0 {
				return numEntries, wasi.ESUCCESS
			}
			n, err := d.file.ReadDirent(d.buffer[:])
			if err != nil {
				return numEntries, wasi.MakeErrno(err)
			}
			if n == 0 {
				return numEntries, wasi.ESUCCESS
			}
			d.offset = 0
			d.length = n
		}

		dirent := (*dirent)(unsafe.Pointer(&d.buffer[d.offset]))

		if (d.offset + int(dirent.reclen)) > d.length {
			d.offset = d.length
			continue
		}

		if d.cookie >= cookie {
			dirEntry := wasi.DirEntry{
				Next:  d.cookie + 1,
				INode: wasi.INode(dirent.ino),
			}

			switch dirent.typ {
			case DT_BLK:
				dirEntry.Type = wasi.BlockDeviceType
			case DT_CHR:
				dirEntry.Type = wasi.CharacterDeviceType
			case DT_DIR:
				dirEntry.Type = wasi.DirectoryType
			case DT_LNK:
				dirEntry.Type = wasi.SymbolicLinkType
			case DT_REG:
				dirEntry.Type = wasi.RegularFileType
			case DT_SOCK:
				dirEntry.Type = wasi.SocketStreamType
			default: // DT_FIFO, DT_WHT, DT_UNKNOWN
				dirEntry.Type = wasi.UnknownType
			}

			i := d.offset + sizeOfDirent
			j := d.offset + int(dirent.reclen)
			dirEntry.Name = d.buffer[i:j:j]

			n := bytes.IndexByte(dirEntry.Name, 0)
			if n >= 0 {
				dirEntry.Name = dirEntry.Name[:n:n]
			}
			entries[numEntries] = dirEntry
			numEntries++

			bufferSizeBytes -= wasi.SizeOfDirent
			bufferSizeBytes -= len(dirEntry.Name)

			if bufferSizeBytes <= 0 {
				return numEntries, wasi.ESUCCESS
			}
		}

		d.offset += int(dirent.reclen)
		d.cookie += 1
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
