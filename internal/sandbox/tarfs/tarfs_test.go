package tarfs_test

import (
	"archive/tar"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/gzip"
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

func TestAlpine(t *testing.T) {
	targz, err := os.Open("testdata/alpine.tar.gz")
	assert.OK(t, err)
	defer targz.Close()

	tarball, err := os.Create(filepath.Join(t.TempDir(), "alpine.tar"))
	assert.OK(t, err)
	defer tarball.Close()

	gunzip, err := gzip.NewReader(targz)
	assert.OK(t, err)
	defer gunzip.Close()

	_, err = io.Copy(tarball, gunzip)
	assert.OK(t, err)

	_, err = tarball.Seek(0, 0)
	assert.OK(t, err)

	fsys, err := tarfs.OpenFile(tarball)
	assert.OK(t, err)

	f, err := fsys.Open("/usr/share/apk/keys/aarch64/alpine-devel@lists.alpinelinux.org-58199dcc.rsa.pub", sandbox.O_RDONLY, 0)
	assert.OK(t, err)
	defer f.Close()

	s, err := f.Stat("", 0)
	assert.OK(t, err)
	assert.Equal(t, s.Size, 451)

	size, memsize, filesize := fsys.Size(), fsys.Memsize(), fsys.Filesize()
	t.Logf("Size     = %d\n", size)
	t.Logf("Memsize  = %d (%.2f%%)\n", memsize, 100*float64(memsize)/float64(size))
	t.Logf("Filesize = %d (%.2f%%)\n", filesize, 100*float64(filesize)/float64(size))
}
