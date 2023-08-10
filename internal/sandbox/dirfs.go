package sandbox

// DirFS constructs a FileSystem instance backed by a directory location on the
// local file system.
//
// The returned FileSystem instance captures the path passed as argument as-is.
// If the path is relative, the resulting FileSystem depends on the program's
// current working directory when opening files.
//
// As long as the directory that the file system is opened on does not change,
// it prevents escaping from it, even in the presence of symbolic links
// referencing paths above the root directory.
func DirFS(path string) FileSystem { return &dirFS{root: path} }
