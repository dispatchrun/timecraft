package tarfs

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
)

// Archive archives the content of the directory at path into a tarball.
func Archive(tarball *tar.Writer, path string) error {
	links := make(map[uint64]string)
	buffer := make([]byte, 32*1024)

	return filepath.Walk(path, func(filePath string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		mode := info.Mode()
		link := ""

		if mode.Type() == fs.ModeSymlink {
			link, err = os.Readlink(filePath)
			if err != nil {
				return err
			}
		}

		h, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		h.Name = filePath[len(path):]
		if h.Name == "" {
			h.Name = "/"
		}

		if !mode.IsDir() {
			if stat, _ := info.Sys().(*syscall.Stat_t); stat != nil && stat.Nlink > 1 && stat.Ino > 0 {
				if link, ok := links[stat.Ino]; ok {
					h.Typeflag = tar.TypeLink
					h.Linkname = link
				} else {
					links[stat.Ino] = h.Name
				}
			}
		}

		if err := tarball.WriteHeader(h); err != nil {
			return &fs.PathError{Op: "archive", Path: h.Name, Err: err}
		}

		switch h.Typeflag {
		case tar.TypeReg, tar.TypeChar, tar.TypeBlock:
			f, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer f.Close()
			n, err := io.CopyBuffer(tarball, f, buffer)
			if err != nil {
				return err
			}
			if size := info.Size(); size != n {
				err := fmt.Errorf("file size and number of bytes written mismatch: size=%d written=%d", size, n)
				return &fs.PathError{Op: "archive", Path: h.Name, Err: err}
			}
		}

		return nil
	})
}
