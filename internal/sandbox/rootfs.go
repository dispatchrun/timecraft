package sandbox

import (
	"errors"
	"io/fs"
	"os"
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

type rootFile struct{ File }

func (f *rootFile) Open(name string, flags int, mode fs.FileMode) (File, error) {
	file, err := resolvePath(f, name, flags, func(dir File, name string) (File, error) {
		return dir.Open(name, flags|O_NOFOLLOW, mode)
	})
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: unwrap(err)}
	}
	return &rootFile{file}, nil
}

func (f *rootFile) Stat(name string, flags int) (FileInfo, error) {
	return withPath2("stat", f, name, flags, func(dir File, name string) (FileInfo, error) {
		info, err := dir.Stat(name, AT_SYMLINK_NOFOLLOW)
		if err == nil {
			if info.Mode.Type() == fs.ModeSymlink && ((flags & AT_SYMLINK_NOFOLLOW) == 0) {
				err = ELOOP
			}
		}
		return info, err
	})
}

func (f *rootFile) Readlink(name string, buf []byte) (int, error) {
	return withPath2("readlink", f, name, AT_SYMLINK_NOFOLLOW, func(dir File, name string) (int, error) {
		return dir.Readlink(name, buf)
	})
}

func (f *rootFile) Chtimes(name string, atime, mtime time.Time, flags int) error {
	return withPath1("chtimes", f, name, flags, func(dir File, name string) error {
		return dir.Chtimes(name, atime, mtime, AT_SYMLINK_NOFOLLOW)
	})
}

func (f *rootFile) Mkdir(name string, mode fs.FileMode) error {
	return withPath1("mkdir", f, name, AT_SYMLINK_NOFOLLOW, func(dir File, name string) error {
		return dir.Mkdir(name, mode)
	})
}

func (f *rootFile) Rmdir(name string) error {
	return withPath1("rmdir", f, name, AT_SYMLINK_NOFOLLOW, File.Rmdir)
}

func (f *rootFile) Rename(oldName string, newDir File, newName string) error {
	f2, ok := newDir.(*rootFile)
	if !ok {
		path1 := joinPath(f.Name(), oldName)
		path2 := joinPath(newDir.Name(), newName)
		return &os.LinkError{Op: "rename", Old: path1, New: path2, Err: EXDEV}
	}
	return withPath3("rename", f, oldName, f2, newName, AT_SYMLINK_NOFOLLOW, File.Rename)
}

func (f *rootFile) Link(oldName string, newDir File, newName string, flags int) error {
	f2, ok := newDir.(*rootFile)
	if !ok {
		path1 := joinPath(f.Name(), oldName)
		path2 := joinPath(newDir.Name(), newName)
		return &os.LinkError{Op: "link", Old: path1, New: path2, Err: EXDEV}
	}
	return withPath3("link", f, oldName, f2, newName, flags, func(oldDir File, oldName string, newDir File, newName string) error {
		return oldDir.Link(oldName, newDir, newName, AT_SYMLINK_NOFOLLOW)
	})
}

func (f *rootFile) Symlink(oldName, newName string) error {
	return withPath1("symlink", f, newName, AT_SYMLINK_NOFOLLOW, func(dir File, name string) error {
		return dir.Symlink(oldName, name)
	})
}

func (f *rootFile) Unlink(name string) error {
	return withPath1("unlink", f, name, AT_SYMLINK_NOFOLLOW, File.Unlink)
}

func withPath1(op string, root *rootFile, path string, flags int, do func(File, string) error) error {
	_, err := resolvePath(root, path, flags, func(dir File, name string) (_ struct{}, err error) {
		err = do(dir, name)
		return
	})
	if err != nil {
		err = &fs.PathError{Op: op, Path: path, Err: unwrap(err)}
	}
	return err
}

func withPath2[R any](op string, root *rootFile, path string, flags int, do func(File, string) (R, error)) (ret R, err error) {
	ret, err = resolvePath(root, path, flags, do)
	if err != nil {
		err = &fs.PathError{Op: op, Path: path, Err: unwrap(err)}
	}
	return ret, err
}

func withPath3(op string, f1 *rootFile, path1 string, f2 *rootFile, path2 string, flags int, do func(File, string, File, string) error) error {
	err := withPath1(op, f1, path1, flags, func(dir1 File, name1 string) error {
		return withPath1(op, f2, path2, flags, func(dir2 File, name2 string) error {
			return do(dir1, name1, dir2, name2)
		})
	})
	if err != nil {
		path1 = joinPath(f1.Name(), path2)
		path2 = joinPath(f2.Name(), path2)
		return &os.LinkError{Op: "link", Old: path1, New: path2, Err: unwrap(err)}
	}
	return nil
}

// resolvePath is the path resolution algorithm which guarantees sandboxing of
// path access in a root FS.
//
// The algorithm walks the path name from f, calling the do function when it
// reaches a path leaf. The function may return ELOOP to indicate that a symlink
// was encountered and must be followed, in which case resolvePath continues
// walking the path at the link target. Any other value or error returned by the
// do function will be returned immediately.
func resolvePath[R any](f *rootFile, name string, flags int, do func(File, string) (R, error)) (ret R, err error) {
	if name == "" {
		return do(f.File, "")
	}
	dir := f.File
	dirToClose := File(nil)

	closeDir := func() {
		if dirToClose != nil {
			dirToClose.Close()
		}
	}
	defer closeDir()

	setCurrentDirectory := func(cd File) {
		closeDir()
		dir, dirToClose = cd, cd
	}

	followSymlinkDepth := 0
	followSymlink := func(symlink, target string) error {
		link, err := readlink(dir, symlink)
		if err != nil {
			// This error may be EINVAL if the file system was modified
			// concurrently and the directory entry was not pointing to a
			// symbolic link anymore.
			return err
		}

		// Limit the maximum number of symbolic links that would be followed
		// during path resolution; this ensures that if we encounter a loop,
		// we will eventually abort resolving the path.
		if followSymlinkDepth == maxFollowSymlink {
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
			d, err := openRoot(f)
			if err != nil {
				return ret, err
			}
			depth = 0
			setCurrentDirectory(d)
		}

		var delta int
		var elem string
		elem, name = walkPath(name)

		switch elem {
		case ".":
		doFile:
			ret, err = do(dir, name)
			if err != nil {
				if !errors.Is(err, ELOOP) || ((flags & O_NOFOLLOW) != 0) {
					return ret, err
				}
				switch err := followSymlink(name, ""); {
				case errors.Is(err, nil):
					continue
				case errors.Is(err, EINVAL):
					goto doFile
				default:
					return ret, err
				}
			}
			return ret, nil

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
				return ret, err
			}
			switch err := followSymlink(elem, name); {
			case errors.Is(err, nil):
				continue
			case errors.Is(err, EINVAL):
				goto openPath
			default:
				return ret, err
			}
		}
		depth += delta
		setCurrentDirectory(d)
	}
}

func openRoot(f *rootFile) (File, error) {
	depth := filePathDepth(f.Name())
	if depth == 0 {
		return f.Open(".", openPathFlags, 0)
	}
	dir := f.File
	dirToClose := File(nil)

	for depth > 0 {
		p, err := dir.Open("..", openPathFlags, 0)
		if err != nil {
			return nil, err
		}
		if dirToClose != nil {
			dirToClose.Close()
		}
		dir, dirToClose = p, p
		depth--
	}

	return dir, nil
}
