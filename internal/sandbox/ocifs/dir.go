package ocifs

import (
	"io"
	"io/fs"
	"strings"
	"sync"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

type dirent struct {
	typ  fs.FileMode
	ino  uint64
	name string
}

type dirbuf struct {
	mutex   sync.Mutex
	index   int
	offset  uint64
	entries []dirent
	strings stringAllocator
}

// init is called autoamtically by readDirent to initialize the directory
// buffer.
//
// Unfortunately, we must load all entries in memory because we need to
// expose a merged view of the entries where entries in upper layers mask
// those in lower layers, and apply whiteout files.
//
// We could reduce the memory footprint by using a k-way merge if we had the
// gurantee that all the layers were producing directory entries sorted by
// name, but this is not one of the guarantees that the sandbox.File.ReadDirent
// method has. Since tarfs sorts entries, and we are using the ocifs as a way
// to merge multiple tarfs layers, we could technically change this requirement
// and make the optimization; the trade off would be on having to buffer the
// entries read by sandbox.DirFS to ensure that they are always sorted... there
// is no free lunch on this one.
func (d *dirbuf) init(files []sandbox.File) error {
	buf := make([]byte, 2*sandbox.PATH_MAX)
	names := make(map[string]struct{}, 16)
	opaque := false

	for _, f := range files {
		for {
			n, err := f.ReadDirent(buf)
			if err != nil {
				return err
			}
			if n == 0 {
				break
			}

			for b := buf[:n]; len(b) > 0; {
				n, typ, ino, _, ent, err := sandbox.ReadDirent(b)
				if err != nil {
					if err == io.ErrShortBuffer {
						break
					}
					return err
				}
				b = b[n:]

				_, seen := names[string(ent)]
				if seen {
					continue
				}

				name := d.strings.makeString(ent)
				switch {
				case name == whiteoutOpaque:
					opaque = true
				case strings.HasPrefix(name, whiteoutPrefix):
					name = name[len(whiteoutPrefix):]
				default:
					d.entries = append(d.entries, dirent{
						typ:  typ,
						ino:  ino,
						name: name,
					})
				}

				names[name] = struct{}{}
			}
		}

		if opaque {
			break
		}
	}

	return nil
}

func (d *dirbuf) reset() {
	d.mutex.Lock()
	if d.index > 0 {
		d.index, d.offset = 0, 0
	}
	d.mutex.Unlock()
}

func (d *dirbuf) readDirent(buf []byte, files []sandbox.File) (int, error) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.index < 0 {
		d.index = 0
		if err := d.init(files); err != nil {
			return 0, err
		}
	}

	n := 0

	for d.index < len(d.entries) && n < len(buf) {
		dirent := &d.entries[d.index]
		wn := sandbox.WriteDirent(buf[n:], dirent.typ, dirent.ino, d.offset, dirent.name)
		n += wn
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
