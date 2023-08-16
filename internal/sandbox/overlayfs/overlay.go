package overlayfs

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"sync"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

type fileOverlay struct {
	mutex  sync.Mutex
	refc   int
	name   string
	lower  sandbox.File
	upper  sandbox.File
	parent *fileOverlay
}

func newFileOverlay(parent *fileOverlay, lower, upper sandbox.File, name string) *fileOverlay {
	overlay := &fileOverlay{
		name:   name,
		lower:  lower,
		upper:  upper,
		parent: parent,
	}
	ref(parent)
	return overlay
}

func (f *fileOverlay) files() []sandbox.File {
	if f.lower != nil && f.upper != nil {
		return []sandbox.File{f.upper, f.lower}
	}
	if f.upper != nil {
		return []sandbox.File{f.upper}
	}
	if f.lower != nil {
		return []sandbox.File{f.lower}
	}
	return nil
}

func (f *fileOverlay) file() sandbox.File {
	if f.upper != nil {
		return f.upper
	} else {
		return f.lower
	}
}

func (f *fileOverlay) makeFile(name string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	lowerFile, err := f.lower.Open(name, sandbox.O_RDONLY, 0)
	if err != nil {
		if errors.Is(err, sandbox.ENOENT) {
			err = nil
		}
		return err
	}
	defer lowerFile.Close()
	lowerInfo, err := lowerFile.Stat("", 0)
	if err != nil {
		return err
	}

	var upperFileName string
	var upperFile sandbox.File
	for i := 0; ; i++ {
		// Another concurrent operation may have attempted to create the file in
		// the upper layer; to allow both operations to progress, we look for a
		// unique temporary file name.
		upperFileName = fmt.Sprintf("%s.%s.%d", tempFilePrefix, name, i)
		upperFile, err = f.upper.Open(upperFileName, sandbox.O_CREAT|sandbox.O_EXCL|sandbox.O_WRONLY, 0)
		if err != nil {
			if errors.Is(err, sandbox.EEXIST) {
				continue
			}
			return err
		}
		break
	}
	defer upperFile.Close()

	success := false
	defer func() {
		if !success {
			_ = f.upper.Unlink(upperFileName)
		}
	}()

	if _, err := lowerFile.CopyRange(0, upperFile, 0, int(lowerInfo.Size)); err != nil {
		return err
	}

	if err := upperFile.Rename(upperFileName, f.upper, name, sandbox.RENAME_NOREPLACE); err != nil {
		// If the target file existed in the upper layer, it indicates that a
		// concurrent operation also attempted to create it and succeeded before
		// this one did. We don't need to do anything, the file exists so we can
		// let the caller continue as if this call had successfully created it.
		if errors.Is(err, sandbox.EEXIST) {
			err = nil
		}
		return err
	}

	success = true
	return nil
}

func (f *fileOverlay) makePath() error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// The upper layer already exists, which means that the path to the
	// directory is already created.
	if f.upper != nil {
		return nil
	}

	if f.parent != nil {
		if err := f.parent.makePath(); err != nil {
			return err
		}
	}

	// TODO: we don't yet support configuring permissions on the directories
	// that are created in the upper layer, but we don't need it for the WASI
	// use case.
	//
	// Note that this is somewhat of a bug because the lower layer may have set
	// restrictive permissions (e.g. not allowing to move files), and mirroring
	// it to the upper layer changes that.
	//
	// When we address this limitation, here are a few things we need to take
	// into account:
	//
	//  - We cannot simply mirror the permissions of the lower directory,
	//    because a directory without write permission would prevent creating
	//    other files or directories in it.
	//
	//  - We could emulate permissions by storing a hidden file in the directory
	//    and implementing permission checks in user space. There are ties to
	//    the concept of user and group that do not exist in WASI and we would
	//    need to replicate.
	//
	//  - We would probably need to add a sandbox.File.Chmod method to support
	//    configuring permissions after the directory is created.
	//
	//  - We might need to support configuring the directory under a different
	//    unique and hidden name prior to moving it to the final location.
	//
	// When we address this issue, we should invest in extensive test suites,
	// permission management is not for the faint of heart!
	if err := f.parent.upper.Mkdir(f.name, 0755); err != nil {
		if !errors.Is(err, sandbox.EEXIST) {
			return err
		}
	}
	d, err := f.parent.upper.Open(f.name, sandbox.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	f.upper = d
	return nil
}

func (f *fileOverlay) ref() {
	f.mutex.Lock()
	f.refc++
	f.mutex.Unlock()
}

func (f *fileOverlay) unref() {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	if f.refc--; f.refc == 0 {
		closeIfNotNil(f.lower)
		closeIfNotNil(f.upper)

		if f.parent != nil {
			f.parent.unref()
		}
	}
}

func ref(f *fileOverlay) {
	if f != nil {
		f.ref()
	}
}

func unref(f *fileOverlay) {
	if f != nil {
		f.unref()
	}
}

func closeIfNotNil(f sandbox.File) {
	if f != nil {
		f.Close()
	}
}

type dirent struct {
	typ  fs.FileMode
	ino  uint64
	name string
}

type dirOverlay struct {
	mutex   sync.Mutex
	index   int
	offset  uint64
	entries []dirent
	error   error
}

func newDirOverlay() *dirOverlay {
	return &dirOverlay{index: -1}
}

func (d *dirOverlay) init(lower, upper sandbox.File, buf []byte) error {
	// The buffer received as argument is usually large enough to be used as
	// scratch space to read entries from the underlying files, so we can avoid
	// allocating a temporary buffer then. We clear the buffer when the function
	// exits to make sure we don't leak details such as whiteout file names.
	const minBufferSize = 2 * sandbox.PATH_MAX
	if len(buf) < minBufferSize {
		buf = make([]byte, minBufferSize)
	} else {
		defer clear(buf)
	}

	// We must keep track of all the names we have seen in the upper layer to
	// be able to mask them from the lower layer.
	names := make(map[string]struct{})
	alloc := stringAllocator{}
	for {
		n, err := upper.ReadDirent(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}
		for b := buf[:n]; len(b) > 0; {
			n, typ, ino, _, ent, err := sandbox.ReadDirent(b)
			if err != nil {
				if err != io.ErrShortBuffer {
					return err
				}
				break
			}

			b = b[n:]
			name := alloc.makeString(ent)

			if strings.HasPrefix(name, whiteoutPrefix) {
				name = name[len(whiteoutPrefix):]
			} else {
				d.entries = append(d.entries, dirent{
					typ:  typ,
					ino:  ino,
					name: name,
				})
			}

			names[name] = struct{}{}
		}
	}

	for {
		n, err := lower.ReadDirent(buf)
		if err != nil {
			return err
		}
		if n == 0 {
			break
		}
		for b := buf[:n]; len(b) > 0; {
			n, typ, ino, _, name, err := sandbox.ReadDirent(b)
			if err != nil {
				if err != io.ErrShortBuffer {
					return err
				}
				break
			}
			if _, exist := names[string(name)]; !exist {
				d.entries = append(d.entries, dirent{
					typ:  typ,
					ino:  ino,
					name: alloc.makeString(name),
				})
			}
			b = b[n:]
		}
	}

	return nil
}

func (d *dirOverlay) reset() {
	d.mutex.Lock()
	if d.index > 0 {
		d.index, d.offset = 0, 0
	}
	d.mutex.Unlock()
}

func (d *dirOverlay) readDirent(lower, upper sandbox.File, buf []byte) (n int, err error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.error != nil {
		return 0, d.error
	}

	if d.index < 0 {
		d.index = 0
		if err := d.init(lower, upper, buf); err != nil {
			d.entries, d.error = nil, err
			return 0, err
		}
	}

	for d.index < len(d.entries) && n < len(buf) {
		dirent := &d.entries[d.index]
		wn := sandbox.WriteDirent(buf[n:], dirent.typ, dirent.ino, d.offset, dirent.name)
		n += wn
		if wn < sandbox.SizeOfDirent(len(dirent.name)) {
			break
		}
		d.index++
		d.offset += uint64(wn)
	}

	return n, nil
}

type stringAllocator struct {
	buf []byte
	off int
}

func (a *stringAllocator) makeString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) > 1024 {
		return string(b)
	}
	if (len(a.buf) - a.off) < len(b) {
		a.buf = make([]byte, 4096)
		a.off = 0
	}
	i := a.off
	j := a.off + len(b)
	a.off = j
	s := a.buf[i:j:j]
	copy(s, b)
	return unsafe.String(&s[0], len(s))
}
