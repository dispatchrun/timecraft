package tarfs

import "golang.org/x/sys/unix"

func copyFileRange(srcfd int, srcoff int64, dstfd int, dstoff int64, length int) (int, error) {
	copied := 0
	for copied < length {
		n, err := unix.CopyFileRange(srcfd, &srcoff, dstfd, &dstoff, length-copied, 0)
		if n > 0 {
			copied += n
		}
		if err != nil && err != unix.EINTR {
			return copied, err
		} else if n == 0 {
			break
		}
	}
	return copied, nil
}
