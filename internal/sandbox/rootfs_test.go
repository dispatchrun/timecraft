package sandbox_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/sandboxtest"
)

func TestRootFS(t *testing.T) {
	t.Run("fstest", func(t *testing.T) {
		testFS(t, sandbox.FS(sandbox.RootFS(sandbox.DirFS("testdata/fstest"))))
	})

	t.Run("sandboxtest", func(t *testing.T) {
		sandboxtest.TestFileSystem(t, func(t *testing.T) sandbox.FileSystem {
			return sandbox.RootFS(sandbox.DirFS(t.TempDir()))
		})
	})

	t.Run("ReadFile", func(t *testing.T) {
		tests := []struct {
			scenario string
			path     string
			flags    int
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
				scenario: "does not follow symlinks when O_NOFOLLOW is set",
				path:     "symlink-to-answer",
				flags:    sandbox.O_NOFOLLOW,
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
				rootFS := sandbox.RootFS(sandbox.DirFS("testdata/rootfs"))
				b, err := sandbox.ReadFile(rootFS, test.path, test.flags)
				if test.err != nil {
					assert.Error(t, err, test.err)
				} else {
					assert.OK(t, err)
					assert.Equal(t, string(b), test.want)
				}
			})
		}
	})
}
