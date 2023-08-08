package sandbox

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/stealthrocket/timecraft/internal/sandbox/fspath"
)

// RootFS wraps the given FileSystem to prevent path resolution from escaping
// its root directory.
//
// RootFS is useful to create a sandbox in combination with a DirFS pointing at
// a directory on the local file system. Operations performed on the RootFS are
// guaranteed not to escape the base directory, even in the presence of symbolic
// links pointing to parent directories of the root.
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

func (f *rootFile) String() string {
	return fmt.Sprintf("&sandbox.rootFile{%v}", f.File)
}

func (f *rootFile) Open(name string, flags int, mode fs.FileMode) (File, error) {
	file, err := ResolvePath(f.File, name, flags, func(dir File, name string) (File, error) {
		return dir.Open(name, flags|O_NOFOLLOW, mode)
	})
	if err != nil {
		return nil, &fs.PathError{Op: "open", Path: name, Err: unwrap(err)}
	}
	return &rootFile{file}, nil
}

func (f *rootFile) Stat(name string, flags int) (FileInfo, error) {
	openFlags := 0
	if (flags & AT_SYMLINK_NOFOLLOW) != 0 {
		openFlags |= O_NOFOLLOW
	}
	return withPath2("stat", f, name, openFlags, func(dir File, name string) (FileInfo, error) {
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

func (f *rootFile) Chtimes(name string, times [2]Timespec, flags int) error {
	return withPath1("chtimes", f, name, flags, func(dir File, name string) error {
		return dir.Chtimes(name, times, AT_SYMLINK_NOFOLLOW)
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
		path1 := fspath.Join(f.Name(), oldName)
		path2 := fspath.Join(newDir.Name(), newName)
		return &os.LinkError{Op: "rename", Old: path1, New: path2, Err: EXDEV}
	}
	return withPath3("rename", f, oldName, f2, newName, AT_SYMLINK_NOFOLLOW, File.Rename)
}

func (f *rootFile) Link(oldName string, newDir File, newName string, flags int) error {
	f2, ok := newDir.(*rootFile)
	if !ok {
		path1 := fspath.Join(f.Name(), oldName)
		path2 := fspath.Join(newDir.Name(), newName)
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
	_, err := ResolvePath(root.File, path, flags, func(dir File, name string) (_ struct{}, err error) {
		err = do(dir, name)
		return
	})
	if err != nil {
		err = &fs.PathError{Op: op, Path: path, Err: unwrap(err)}
	}
	return err
}

func withPath2[R any](op string, root *rootFile, path string, flags int, do func(File, string) (R, error)) (ret R, err error) {
	ret, err = ResolvePath(root.File, path, flags, do)
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
		path1 = fspath.Join(f1.Name(), path2)
		path2 = fspath.Join(f2.Name(), path2)
		return &os.LinkError{Op: "link", Old: path1, New: path2, Err: unwrap(err)}
	}
	return nil
}

// ResolvePath is the path resolution algorithm which guarantees sandboxing of
// path access in a root FS.
//
// The algorithm walks the path name from f, calling the do function when it
// reaches a path leaf. The function may return ELOOP to indicate that a symlink
// was encountered and must be followed, in which case ResolvePath continues
// walking the path at the link target. Any other value or error returned by the
// do function will be returned immediately.
func ResolvePath[R any](dir File, name string, flags int, do func(File, string) (R, error)) (ret R, err error) {
	if name == "" {
		return do(dir, "")
	}
	if fspath.HasTrailingSlash(name) {
		flags |= O_DIRECTORY
	}

	var lastOpenDir File
	defer func() { closeFileIfNotNil(lastOpenDir) }()

	setCurrentDirectory := func(cd File) {
		closeFileIfNotNil(lastOpenDir)
		dir, lastOpenDir = cd, cd
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

	depth := fspath.Depth(dir.Name())
	for {
		if fspath.IsAbs(name) {
			if name = fspath.TrimLeadingSlash(name); name == "" {
				name = "."
			}
			d, err := openRoot(dir)
			if err != nil {
				return ret, err
			}
			depth = 0
			setCurrentDirectory(d)
		}

		var delta int
		var elem string
		elem, name = fspath.Walk(name)

		if name == "" {
		doFile:
			ret, err = do(dir, elem)
			if err != nil {
				if !errors.Is(err, ELOOP) || ((flags & O_NOFOLLOW) != 0) {
					return ret, err
				}
				switch err := followSymlink(elem, ""); {
				case errors.Is(err, nil):
					continue
				case errors.Is(err, EINVAL):
					goto doFile
				default:
					return ret, err
				}
			}
			return ret, nil
		}

		if elem == "." {
			continue
		}

		if elem == ".." {
			// This check ensures that we cannot escape the root of the file
			// system when accessing a parent directory.
			if depth == 0 {
				continue
			}
			delta = -1
		} else {
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

func openRoot(dir File) (File, error) {
	depth := fspath.Depth(dir.Name())
	if depth == 0 {
		return dir.Open(".", openPathFlags, 0)
	}

	var lastOpenDir File
	for depth > 0 {
		p, err := dir.Open("..", openPathFlags, 0)
		if err != nil {
			return nil, err
		}
		closeFileIfNotNil(lastOpenDir)
		dir, lastOpenDir = p, p
		depth--
	}

	return dir, nil
}

func closeFileIfNotNil(f File) {
	if f != nil {
		f.Close()
	}
}
