package tarfs_test

import (
	"archive/tar"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/sandboxtest"
	"github.com/stealthrocket/timecraft/internal/sandbox/tarfs"
)

func TestTarFS(t *testing.T) {
	t.Run("fs.FS", func(t *testing.T) {
		sandboxtest.TestFS(t, func(t *testing.T, path string) fs.FS {
			return sandbox.FS(makeTarFS(t, path))
		})
	})

	sandboxtest.TestRootFS(t, makeTarFS)
}

func makeTarFS(t *testing.T, path string) sandbox.FileSystem {
	tmp := t.TempDir()

	f, err := os.Create(filepath.Join(tmp, "fs.tar"))
	assert.OK(t, err)

	w := tar.NewWriter(f)
	assert.OK(t, tarfs.Archive(w, path))
	assert.OK(t, w.Close())

	_, err = f.Seek(0, 0)
	assert.OK(t, err)

	fsys, err := tarfs.OpenFile(f)
	assert.OK(t, err)
	return fsys
}
