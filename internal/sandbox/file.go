package sandbox

import (
	"context"
	"io"
	"io/fs"
	"path"

	"github.com/stealthrocket/fsinfo"
	"github.com/stealthrocket/wasi-go"
)

type file struct {
	defaultFile
	fsys fs.FS
	file fs.File
	path string
}

func openFile(fsys fs.FS, path string) (*file, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, err
	}
	return &file{fsys: fsys, file: f, path: path}, nil
}

func (f *file) hook(ev wasi.EventType, ch chan<- struct{}) {
	// files are always ready for both reading and writing
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (f *file) poll(ev wasi.EventType) bool {
	return true
}

func (f *file) FDClose(ctx context.Context) wasi.Errno {
	return wasi.MakeErrno(f.file.Close())
}

func (f *file) FDRead(ctx context.Context, iovs []wasi.IOVec) (size wasi.Size, errno wasi.Errno) {
	if len(iovs) == 0 {
		return 0, wasi.EINVAL
	}
	for _, iov := range iovs {
		n, err := io.ReadFull(f.file, iov)
		size += wasi.Size(n)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			return size, wasi.MakeErrno(err)
		}
	}
	return size, wasi.ESUCCESS
}

func (f *file) FDPread(ctx context.Context, iovs []wasi.IOVec, offset wasi.FileSize) (size wasi.Size, errno wasi.Errno) {
	if len(iovs) == 0 {
		return 0, wasi.EINVAL
	}
	r, ok := f.file.(io.ReaderAt)
	if !ok {
		return 0, wasi.ESPIPE
	}
	for _, iov := range iovs {
		n, err := r.ReadAt(iov, int64(offset))
		size += wasi.Size(n)
		offset += wasi.FileSize(n)
		if err != nil {
			if err == io.EOF {
				break
			}
			return size, wasi.MakeErrno(err)
		}
		if n < len(iov) {
			return size, wasi.EIO
		}
	}
	return size, wasi.ESUCCESS
}

func (f *file) FDSeek(ctx context.Context, delta wasi.FileDelta, whence wasi.Whence) (size wasi.FileSize, errno wasi.Errno) {
	s, ok := f.file.(io.Seeker)
	if !ok {
		return 0, wasi.ESPIPE
	}
	offset, err := s.Seek(int64(delta), int(whence))
	return wasi.FileSize(offset), wasi.MakeErrno(err)
}

func (f *file) FDFileStatGet(ctx context.Context) (stat wasi.FileStat, errno wasi.Errno) {
	info, err := f.file.Stat()
	if err != nil {
		return stat, wasi.MakeErrno(err)
	}
	return makeFileStat(info), wasi.ESUCCESS
}

func (f *file) PathOpen(ctx context.Context, lookupFlags wasi.LookupFlags, path string, openFlags wasi.OpenFlags, rightsBase, rightsInheriting wasi.Rights, fdFlags wasi.FDFlags) (anyFile, wasi.Errno) {
	if !lookupFlags.Has(wasi.SymlinkFollow) {
		return nil, wasi.EINVAL
	}
	if fdFlags != 0 {
		return nil, wasi.EINVAL
	}
	if rightsBase.Has(wasi.FDWriteRight) {
		return nil, wasi.EPERM
	}
	path = f.joinPath(path)
	open, err := f.fsys.Open(path)
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	newFile := &file{
		fsys: f.fsys,
		file: open,
		path: path,
	}
	return newFile, wasi.ESUCCESS
}

func (f *file) PathFileStatGet(ctx context.Context, flags wasi.LookupFlags, path string) (wasi.FileStat, wasi.Errno) {
	// TODO: handle lookup flags
	info, err := fs.Stat(f.fsys, f.joinPath(path))
	if err != nil {
		return wasi.FileStat{}, wasi.MakeErrno(err)
	}
	return makeFileStat(info), wasi.ESUCCESS
}

func (f *file) FDOpenDir(ctx context.Context) (wasi.Dir, wasi.Errno) {
	dirents, err := fs.ReadDir(f.fsys, f.path)
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return &dir{dirents}, wasi.ESUCCESS
}

func (f *file) joinPath(p string) string {
	return path.Join(f.path, p)
}

func makeFileType(fileMode fs.FileMode) wasi.FileType {
	switch fileMode.Type() {
	case 0:
		return wasi.RegularFileType
	case fs.ModeDevice:
		return wasi.BlockDeviceType
	case fs.ModeDevice | fs.ModeCharDevice:
		return wasi.CharacterDeviceType
	case fs.ModeDir:
		return wasi.DirectoryType
	case fs.ModeSocket:
		return wasi.SocketStreamType // BUG: we can't differentiate between stream and dgram sockets
	case fs.ModeSymlink:
		return wasi.SymbolicLinkType
	default:
		return wasi.UnknownType
	}
}

func makeFileStat(info fs.FileInfo) wasi.FileStat {
	return wasi.FileStat{
		FileType:   makeFileType(info.Mode()),
		Device:     wasi.Device(fsinfo.Device(info)),
		INode:      wasi.INode(fsinfo.Ino(info)),
		NLink:      wasi.LinkCount(fsinfo.Nlink(info)),
		Size:       wasi.FileSize(info.Size()),
		AccessTime: wasi.Timestamp(fsinfo.AccessTime(info).UnixNano()),
		ModifyTime: wasi.Timestamp(fsinfo.ModTime(info).UnixNano()),
		ChangeTime: wasi.Timestamp(fsinfo.ChangeTime(info).UnixNano()),
	}
}

type dir struct {
	entries []fs.DirEntry
}

func (d *dir) FDReadDir(ctx context.Context, entries []wasi.DirEntry, cookie wasi.DirCookie, bufferSizeBytes int) (n int, errno wasi.Errno) {
	if cookie < 0 || cookie > wasi.DirCookie(len(d.entries)) {
		return -1, wasi.EINVAL
	}
	for _, ent := range d.entries[cookie:] {
		if bufferSizeBytes <= 0 {
			break
		}
		if n == len(entries) {
			break
		}
		name := ent.Name()
		entries[n] = wasi.DirEntry{
			Next: cookie + wasi.DirCookie(n) + 1,
			Type: makeFileType(ent.Type()),
			Name: []byte(name),
		}
		bufferSizeBytes -= wasi.SizeOfDirent + len(name)
		n++
	}
	return n, wasi.ESUCCESS
}

func (d *dir) FDCloseDir(ctx context.Context) wasi.Errno {
	d.entries = nil
	return wasi.ESUCCESS
}
