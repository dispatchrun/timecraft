//go:build !linux

package tarfs

import "github.com/stealthrocket/timecraft/internal/sandbox"

func copyFileRange(srcfd int, srcoff int64, dstfd int, dstoff int64, length int) (int, error) {
	return -1, sandbox.ENOSYS
}
