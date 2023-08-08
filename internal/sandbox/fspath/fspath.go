// Package fspath is similar to the standard path package but provides functions
// that are more useful for path manipulation in the presence of symbolic links.
package fspath

// Depth returns the depth of a path. The root "/" has depth zero.
func Depth(path string) (depth int) {
	for {
		path = TrimLeadingSlash(path)
		if path == "" {
			return depth
		}
		i := IndexSlash(path)
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

// Join is similar to path.Join but is simplified to only join two paths and
// avoid cleaning parent directory references in the paths.
func Join(dir, name string) string {
	buf := make([]byte, 0, 256)
	buf = AppendClean(buf, dir)
	if name = TrimLeadingSlash(name); name != "" {
		if !HasTrailingSlash(string(buf)) {
			buf = append(buf, '/')
		}
		buf = AppendClean(buf, name)
	}
	return string(buf)
}

// Clean is like path.Clean but it preserves parent directory references;
// this is necessary to ensure that symbolic links aren't erased from walking
// the path.
func Clean(path string) string {
	buf := make([]byte, 0, 256)
	buf = AppendClean(buf, path)
	return string(buf)
}

// AppendClean is like cleanPath but it appends the result to the byte slice
// passed as first argument.
func AppendClean(buf []byte, path string) []byte {
	if len(path) == 0 {
		return buf
	}

	type region struct {
		off, end int
	}

	elems := make([]region, 0, 16)
	if IsAbs(path) {
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
	if HasTrailingSlash(path) {
		buf = append(buf, '/')
	}
	return buf
}

// IndexSlash is like strings.IndexByte(path, '/') but the function is simple
// enough to be inlined, which is a measurable improvement since it gets called
// very often by the other routines in this file.
func IndexSlash(path string) int {
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			return i
		}
	}
	return -1
}

// Walk separates the next path element from the rest of the path.
func Walk(path string) (elem, name string) {
	i := IndexSlash(path)
	if i < 0 {
		return path, ""
	}
	if i == 0 {
		i = 1
	}
	return path[:i], TrimLeadingSlash(path[i:])
}

func HasTrailingSlash(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '/'
}

func TrimLeadingSlash(s string) string {
	i := 0
	for i < len(s) && s[i] == '/' {
		i++
	}
	return s[i:]
}

func TrimTrailingSlash(s string) string {
	i := len(s)
	for i > 0 && s[i-1] == '/' {
		i--
	}
	return s[:i]
}

func IsAbs(path string) bool {
	return len(path) > 0 && path[0] == '/'
}
