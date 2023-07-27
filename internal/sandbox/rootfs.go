package sandbox

import (
	"errors"
	"io/fs"
	"path"
	"time"
)

func RootFS(fsys FileSystem) FileSystem {
	return &rootFS{fsys}
}

type rootFS struct{ base FileSystem }

func (fsys *rootFS) Open(name string, flags int, mode fs.FileMode) (File, error) {
	f, err := OpenRoot(fsys.base)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return (&rootFile{f}).Open(name, flags, mode)
}

type nopFileCloser struct{ File }

func (nopFileCloser) Close() error { return nil }

type rootFile struct{ File }

func (f *rootFile) Open(name string, flags int, mode fs.FileMode) (File, error) {
	file, err := f.open(name, flags, mode)
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: unwrapPathError(err)}
	}
	return &rootFile{file}, nil
}

func (f *rootFile) open(name string, flags int, mode fs.FileMode) (File, error) {
	dir := File(nopFileCloser{f.File})
	defer func() { dir.Close() }()
	setCurrentDirectory := func(cd File) {
		dir.Close()
		dir = cd
	}

	followSymlinkDepth := 0
	followSymlink := func(symlink, target string) error {
		link, err := dir.Readlink(symlink)
		if err != nil {
			// This error may be EINVAL if the file system was modified
			// concurrently and the directory entry was not pointing to a
			// symbolic link anymore.
			return err
		}

		// Limit the maximum number of symbolic links that would be followed
		// during path resolution; this ensures that if we encounter a loop,
		// we will eventually abort resolving the path.
		if followSymlinkDepth == MaxFollowSymlink {
			return ELOOP
		}
		followSymlinkDepth++

		if target != "" {
			name = link + "/" + target
		} else {
			name = link
		}
		return nil
	}

	depth := filePathDepth(f.Name())
	for {
		if isAbs(name) {
			if name = trimLeadingSlash(name); name == "" {
				name = "."
			}
			d, err := f.openRoot()
			if err != nil {
				return nil, err
			}
			depth = 0
			setCurrentDirectory(d)
		}

		var delta int
		var elem string
		elem, name = walkPath(name)

		switch elem {
		case ".":
		openFile:
			file, err := dir.Open(name, flags|O_NOFOLLOW, mode)
			if err != nil {
				if !errors.Is(err, ELOOP) || ((flags & O_NOFOLLOW) != 0) {
					return nil, err
				}
				switch err := followSymlink(name, ""); err {
				case nil:
					continue
				case EINVAL:
					goto openFile
				default:
					return nil, err
				}
			}
			return file, nil
		case "..":
			// This check ensures that we cannot escape the root of the file
			// system when accessing a parent directory.
			if depth == 0 {
				continue
			}
			delta = -1
		default:
			delta = +1
		}

	openPath:
		d, err := dir.Open(elem, openPathFlags, 0)
		if err != nil {
			if !errors.Is(err, ENOTDIR) {
				return nil, err
			}
			switch err := followSymlink(elem, name); err {
			case nil:
				continue
			case EINVAL:
				goto openPath
			default:
				return nil, err
			}
		}
		depth += delta
		setCurrentDirectory(d)
	}
}

func (f *rootFile) openRoot() (File, error) {
	depth := filePathDepth(f.Name())
	if depth == 0 {
		return f.Open(".", openPathFlags, 0)
	}
	dir := File(nopFileCloser{f.File})
	for depth > 0 {
		p, err := dir.Open("..", openPathFlags, 0)
		if err != nil {
			return nil, err
		}
		dir.Close()
		dir = p
	}
	return dir, nil
}

func (f *rootFile) Lstat(name string) (fs.FileInfo, error) {
	return atPath2(f, "stat", name, File.Lstat)
}

func (f *rootFile) Readlink(name string) (string, error) {
	return atPath2(f, "readlink", name, File.Readlink)
}

func (f *rootFile) Chtimes(name string, atime, mtime time.Time) error {
	if name == "" {
		return f.File.Chtimes("", atime, mtime)
	}
	return atPath1(f, "chtimes", name, func(dir File, name string) error {
		return dir.Chtimes(name, atime, mtime)
	})
}

func (f *rootFile) Mkdir(name string, mode uint32) error {
	return atPath1(f, "mkdir", name, func(dir File, name string) error {
		return dir.Mkdir(name, mode)
	})
}

func (f *rootFile) Rmdir(name string) error {
	return atPath1(f, "rmdir", name, File.Rmdir)
}

func (f *rootFile) Rename(oldName string, newDir File, newName string) error {
	return ENOSYS
}

func (f *rootFile) Link(oldName string, newDir File, newName string) error {
	return ENOSYS
}

func (f *rootFile) Symlink(oldName, newName string) error {
	return atPath1(f, "symlink", newName, func(dir File, name string) error {
		return dir.Symlink(oldName, name)
	})
}

func (f *rootFile) Unlink(name string) error {
	return atPath1(f, "unlink", name, File.Unlink)
}

func atPath1(root *rootFile, op, name string, do func(File, string) error) error {
	dir, base := path.Split(name)
	if dir == "" {
		return do(root.File, base)
	}
	d, err := root.Open(dir, openPathFlags, 0)
	if err != nil {
		return &fs.PathError{Op: op, Path: name, Err: unwrapPathError(err)}
	}
	defer d.Close()
	return do(d.(*rootFile).File, base)
}

func atPath2[F func(File, string) (R, error), R any](root *rootFile, op, name string, do F) (ret R, err error) {
	dir, base := path.Split(name)
	if dir == "" {
		return do(root.File, base)
	}
	d, err := root.Open(dir, openPathFlags, 0)
	if err != nil {
		return ret, &fs.PathError{Op: op, Path: name, Err: unwrapPathError(err)}
	}
	defer d.Close()
	return do(d.(*rootFile).File, base)
}
