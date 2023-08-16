package overlayfs_test

import (
	"io/fs"
	"testing"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/overlayfs"
	"github.com/stealthrocket/timecraft/internal/sandbox/sandboxtest"
)

func TestOverlayFS(t *testing.T) {
	t.Run("sandbox.FileSystem", func(t *testing.T) {
		sandboxtest.TestFileSystem(t, func(t *testing.T) sandbox.FileSystem {
			lower := sandbox.DirFS(t.TempDir())
			upper := sandbox.DirFS(t.TempDir())
			return overlayfs.New(lower, upper)
		})
	})

	tests := []struct {
		scenario string
		makeFS   func(*testing.T, string) sandbox.FileSystem
	}{
		{
			scenario: "FS in lower layer",
			makeFS: func(t *testing.T, path string) sandbox.FileSystem {
				return overlayfs.New(
					sandbox.DirFS(path),
					sandbox.DirFS(t.TempDir()),
				)
			},
		},

		{
			scenario: "FS in upper layer",
			makeFS: func(t *testing.T, path string) sandbox.FileSystem {
				return overlayfs.New(
					sandbox.DirFS(t.TempDir()),
					sandbox.DirFS(path),
				)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			t.Run("fs.FS", func(t *testing.T) {
				sandboxtest.TestFS(t, func(t *testing.T, path string) fs.FS {
					return sandbox.FS(test.makeFS(t, path))
				})
			})

			sandboxtest.TestRootFS(t, func(t *testing.T, path string) sandbox.FileSystem {
				return test.makeFS(t, path)
			})
		})
	}
}
