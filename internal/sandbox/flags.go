package sandbox

import (
	"sort"
	"strings"
)

// OpenFlags is a bitset of flags that can be passed to the Open method of
// File and FileSystem values.
type OpenFlags int

func makeOpenFlags(sysFlags int) OpenFlags {
	return OpenFlags(sysFlags)
}

func (openFlags OpenFlags) String() string {
	var names []string

	switch openFlags & (O_RDWR | O_WRONLY | O_RDONLY) {
	case O_RDWR:
		names = append(names, "O_RDWR")
	case O_WRONLY:
		names = append(names, "O_WRONLY")
	}

	for _, f := range [...]struct {
		flag OpenFlags
		name string
	}{
		{O_APPEND, "O_APPEND"},
		{O_CREAT, "O_CREAT"},
		{O_EXCL, "O_EXCL"},
		{O_SYNC, "O_SYNC"},
		{O_TRUNC, "O_TRUNC"},
		{O_DIRECTORY, "O_DIRECTORY"},
		{O_NOFOLLOW, "O_NOFOLLOW"},
		{O_NONBLOCK, "O_NONBLOCK"},
	} {
		if (openFlags & f.flag) != 0 {
			names = append(names, f.name)
		}
	}

	if len(names) == 0 {
		names = append(names, "O_RDONLY")
	}

	sort.Strings(names)
	return strings.Join(names, "|")
}

func (openFlags OpenFlags) LookupFlags() LookupFlags {
	if (openFlags & O_NOFOLLOW) != 0 {
		return AT_SYMLINK_NOFOLLOW
	} else {
		return 0
	}
}

func (openFlags OpenFlags) sysFlags() int {
	return int(openFlags)
}

// LookupFlags is a bitset of flags that can be passed to methods of File and
// FileSystem values to customize the behavior of file name lookups.
type LookupFlags int

func (lookupFlags LookupFlags) String() string {
	if (lookupFlags & AT_SYMLINK_NOFOLLOW) != 0 {
		return "AT_SYMLINK_NOFOLLOW"
	} else {
		return "AT_SYMLINK_FOLLOW"
	}
}

func (lookupFlags LookupFlags) OpenFlags() OpenFlags {
	if (lookupFlags & AT_SYMLINK_NOFOLLOW) != 0 {
		return O_NOFOLLOW
	} else {
		return 0
	}
}

func (lookupFlags LookupFlags) sysFlags() int {
	return int(lookupFlags)
}
