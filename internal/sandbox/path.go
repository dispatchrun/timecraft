package sandbox

import (
	"strings"
)

// filePathDepth returns the depth of a path. The root "/" has depth zero.
func filePathDepth(path string) (depth int) {
	for {
		path = trimLeadingSlash(path)
		if path == "" {
			return depth
		}
		depth++
		i := strings.IndexByte(path, '/')
		if i < 0 {
			return depth
		}
		path = path[i:]
	}
}

// joinPath is similar to path.Join but is simplified to assumed that the
// paths passed as arguments hare already clean.
func joinPath(dir, name string) string {
	return trimTrailingSlash(dir) + "/" + trimLeadingSlash(name)
}

// cleanPath is like path.Clean but it preserves parent directory references;
// this is necessary to ensure that sybolic links aren't erased from walking
// the path.
func cleanPath(path string) string {
	if path == "" {
		return ""
	}
	parts := make([]string, 0, 16)
	if isAbs(path) {
		parts = append(parts, "")
	}
	for {
		path = trimLeadingSlash(path)
		if path == "" {
			if len(parts) == 0 {
				return "."
			}
			if len(parts) == 1 && parts[0] == "" {
				return "/"
			}
			return strings.Join(parts, "/")
		}
		var elem string
		if i := strings.IndexByte(path, '/'); i < 0 {
			elem = path
			path = ""
		} else {
			elem = path[:i]
			path = path[i:]
		}
		if elem != "." {
			parts = append(parts, elem)
		}
	}
}

func walkPath(path string) (elem, name string) {
	path = trimLeadingSlash(path)
	path = trimTrailingSlash(path)
	i := strings.IndexByte(path, '/')
	if i < 0 {
		return ".", path
	} else {
		return path[:i], trimLeadingSlash(path[i:])
	}
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
