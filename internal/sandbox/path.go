package sandbox

import "path"

// filePathDepth returns the depth of a path. The root "/" has depth zero.
func filePathDepth(path string) (depth int) {
	for {
		path = trimLeadingSlash(path)
		if path == "" {
			return depth
		}
		i := indexSlash(path)
		if i < 0 {
			i = len(path)
		}
		switch path[:i] {
		case ".":
		case "..":
			if depth > 0 {
				depth--
			}
		default:
			depth++
		}
		path = path[i:]
	}
}

// joinPath is similar to path.Join but is simplified to assumed that the
// paths passed as arguments hare already clean.
func joinPath(dir, name string) string {
	if dir == "" {
		return name
	}
	name = trimLeadingSlash(name)
	if name == "" {
		return dir
	}
	return trimTrailingSlash(dir) + "/" + name
}

// cleanPath is like path.Clean but it preserves parent directory references;
// this is necessary to ensure that sybolic links aren't erased from walking
// the path.
func cleanPath(path string) string {
	buf := make([]byte, 0, 256)
	buf = appendCleanPath(buf, path)
	return string(buf)
}

// appendCleanPath is like cleanPath but it appends the result to the byte
// slice passed as first argument.
func appendCleanPath(buf []byte, path string) []byte {
	if len(path) == 0 {
		return buf
	}

	type region struct {
		off, end int
	}

	elems := make([]region, 0, 16)
	if isAbs(path) {
		elems = append(elems, region{})
	}

	i := 0
	for {
		for i < len(path) && path[i] == '/' {
			i++
		}
		if i == len(path) {
			break
		}
		j := i
		for j < len(path) && path[j] != '/' {
			j++
		}
		if path[i:j] != "." {
			elems = append(elems, region{off: i, end: j})
		}
		i = j
	}

	if len(elems) == 0 {
		return append(buf, '.')
	}
	if len(elems) == 1 && elems[0] == (region{}) {
		return append(buf, '/')
	}
	for i, elem := range elems {
		if i != 0 {
			buf = append(buf, '/')
		}
		buf = append(buf, path[elem.off:elem.end]...)
	}
	if hasTrailingSlash(path) {
		buf = append(buf, '/')
	}
	return buf
}

func indexSlash(path string) int {
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			return i
		}
	}
	return -1
}

func splitPath(dirpath string) (dir, base string) {
	return path.Split(dirpath)
}

func walkPath(path string) (elem, name string) {
	path = trimLeadingSlash(path)
	path = trimTrailingSlash(path)
	i := indexSlash(path)
	if i < 0 {
		return ".", path
	} else {
		return path[:i], trimLeadingSlash(path[i:])
	}
}

func hasTrailingSlash(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '/'
}

func trimLeadingSlash(s string) string {
	i := 0
	for i < len(s) && s[i] == '/' {
		i++
	}
	return s[i:]
}

func trimTrailingSlash(s string) string {
	i := len(s)
	for i > 0 && s[i-1] == '/' {
		i--
	}
	return s[:i]
}

func isAbs(path string) bool {
	return len(path) > 0 && path[0] == '/'
}
