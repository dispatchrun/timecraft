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

func (of OpenFlags) String() string {
	var names []string

	switch of & (O_RDWR | O_WRONLY | O_RDONLY) {
	case O_RDWR:
		names = append(names, "O_RDWR")
	case O_WRONLY:
		names = append(names, "O_WRONLY")
	}

	names = appendFlagNames(names, of, []flagName[OpenFlags]{
		{O_APPEND, "O_APPEND"},
		{O_CREAT, "O_CREAT"},
		{O_EXCL, "O_EXCL"},
		{O_SYNC, "O_SYNC"},
		{O_TRUNC, "O_TRUNC"},
		{O_DIRECTORY, "O_DIRECTORY"},
		{O_NOFOLLOW, "O_NOFOLLOW"},
		{O_NONBLOCK, "O_NONBLOCK"},
	})

	if len(names) == 0 {
		names = append(names, "O_RDONLY")
	}

	return joinFlagNames(names)
}

func (of OpenFlags) LookupFlags() LookupFlags {
	if (of & O_NOFOLLOW) != 0 {
		return AT_SYMLINK_NOFOLLOW
	} else {
		return 0
	}
}

func (of OpenFlags) sysFlags() int {
	return int(of)
}

// LookupFlags is a bitset of flags that can be passed to methods of File and
// FileSystem values to customize the behavior of file name lookups.
type LookupFlags int

func (lf LookupFlags) String() string {
	return formatFlagNames(lf, []flagName[LookupFlags]{
		{AT_SYMLINK_NOFOLLOW, "AT_SYMLINK_NOFOLLOW"},
	})
}

func (lf LookupFlags) OpenFlags() OpenFlags {
	if (lf & AT_SYMLINK_NOFOLLOW) != 0 {
		return O_NOFOLLOW
	} else {
		return 0
	}
}

func (lf LookupFlags) sysFlags() int {
	return int(lf)
}

// RenameFlags is a bitset of flags passed to the File.Rename method to
// configure the behavior of the rename operation.
type RenameFlags int

func (rf RenameFlags) String() string {
	return formatFlagNames(rf, []flagName[RenameFlags]{
		{RENAME_EXCHANGE, "RENAME_EXCHANGE"},
		{RENAME_NOREPLACE, "RENAME_NOREPLACE"},
	})
}

func (rf RenameFlags) sysFlags() int {
	return int(rf)
}

type flag interface {
	~int
}

type flagName[F flag] struct {
	flag F
	name string
}

func formatFlagNames[F flag](flags F, flagNames []flagName[F]) string {
	return joinFlagNames(appendFlagNames(nil, flags, flagNames))
}

func appendFlagNames[F flag](names []string, flags F, flagNames []flagName[F]) []string {
	for _, f := range flagNames {
		if (flags & f.flag) != 0 {
			names = append(names, f.name)
		}
	}
	return names
}

func joinFlagNames(names []string) string {
	if len(names) == 0 {
		return "0"
	}
	sort.Strings(names)
	return strings.Join(names, "|")
}
