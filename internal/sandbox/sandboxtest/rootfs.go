package sandboxtest

import (
	"errors"
	"io/fs"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

// TestRootFS is a test suite designed to validate the implementation of root
// file systems. Such file systems are expected to guarantee that the resolution
// of file paths never escapes the root, even in the presence of symbolic links.
func TestRootFS(t *testing.T, makeRootFS func(*testing.T, string) sandbox.FileSystem) {
	tests := []struct {
		scenario string
		path     string
		flags    sandbox.LookupFlags
		want     string
		err      error
	}{
		{
			scenario: "regular file in the top-level directory",
			path:     "answer",
			want:     "42\n",
		},

		{
			scenario: "regular file in a sub-directory",
			path:     "tmp/message",
			want:     "hello world\n",
		},

		{
			scenario: "cannot escape the root directory via relative path",
			path:     "../../tmp/message",
			want:     "hello world\n",
		},

		{
			scenario: "follows symlinks to files in the same directory",
			path:     "symlink-to-answer",
			want:     "42\n",
		},

		{
			scenario: "follows symlinks to files in a sub-directory",
			path:     "symlink-to-message",
			want:     "hello world\n",
		},

		{
			scenario: "follows symlinks to files in a parent directory",
			path:     "tmp/symlink-to-answer",
			want:     "42\n",
		},

		{
			scenario: "follows absolute symlinks to files in the same directory",
			path:     "absolute-symlink-to-answer",
			want:     "42\n",
		},

		{
			scenario: "follows absolute symlinks to files in a parent directory",
			path:     "tmp/absolute-symlink-to-answer",
			want:     "42\n",
		},

		{
			scenario: "follows absolute symlinks to files in a sub-directory",
			path:     "absolute-symlink-to-message",
			want:     "hello world\n",
		},

		{
			scenario: "follows absolute symlinks to directories",
			path:     "absolute-symlink-to-tmp/message",
			want:     "hello world\n",
		},

		{
			scenario: "does not follow symlinks when AT_SYMLINK_NOFOLLOW is set",
			path:     "symlink-to-answer",
			flags:    sandbox.AT_SYMLINK_NOFOLLOW,
			err:      sandbox.ELOOP,
		},

		{
			scenario: "does not follow dangling symlinks",
			path:     "symlink-to-nowhere",
			err:      sandbox.ENOENT,
		},

		{
			scenario: "does not follow absolute dangling symlinks",
			path:     "absolute-symlink-to-nowhere",
			err:      sandbox.ENOENT,
		},

		{
			scenario: "follows relative symlinks to files in the same directory",
			path:     "relative-symlink-to-answer",
			want:     "42\n",
		},

		{
			scenario: "follows relative symlinks to files in a sub-directory",
			path:     "relative-symlink-to-message",
			want:     "hello world\n",
		},

		{
			scenario: "follows relative symlinks to a sub-directory",
			path:     "relative-symlink-to-tmp/message",
			want:     "hello world\n",
		},

		{
			scenario: "does not follow relative dangling symlinks",
			path:     "relative-symlink-to-nowhere",
			err:      sandbox.ENOENT,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			path := testdata("rootfs")
			rootFS := makeRootFS(t, path)

			b, err := sandbox.ReadFile(rootFS, test.path, test.flags)
			if test.err != nil {
				assert.Error(t, err, test.err)
			} else {
				assert.OK(t, err)
				assert.Equal(t, string(b), test.want)
			}

			switch {
			case errors.Is(err, sandbox.ENOENT):
				_, err := sandbox.Stat(rootFS, test.path)
				assert.Error(t, err, sandbox.ENOENT)

			case errors.Is(err, sandbox.ELOOP):
				s, err := sandbox.Lstat(rootFS, test.path)
				assert.OK(t, err)
				assert.Equal(t, s.Mode.Type(), fs.ModeSymlink)

			default:
				s, err := sandbox.Stat(rootFS, test.path)
				assert.OK(t, err)
				assert.Equal(t, s.Mode.Type(), 0)
			}
		})
	}
}
