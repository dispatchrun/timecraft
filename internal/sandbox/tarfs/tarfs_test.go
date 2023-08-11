package tarfs_test

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
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

	t.Run("CopyFileRange", func(t *testing.T) {
		tmp := t.TempDir()
		tmpFS := sandbox.DirFS(tmp)
		assert.OK(t, sandbox.WriteFile(tmpFS, "src", []byte("Hello World!"), 0644))

		tarFS := makeTarFS(t, tmp)

		srcFile, err := sandbox.Open(tarFS, "src")
		assert.OK(t, err)
		defer srcFile.Close()

		dstFile, err := sandbox.Create(tmpFS, "dst", 0644)
		assert.OK(t, err)
		defer dstFile.Close()

		_, err = srcFile.CopyFileRange(0, dstFile, 0, 12)
		assert.OK(t, err)
		assert.OK(t, dstFile.Close())

		b, err := sandbox.ReadFile(tmpFS, "dst", 0)
		assert.OK(t, err)
		assert.Equal(t, string(b), "Hello World!")
	})
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

	// sha256 hash of the file we read below, we validate that the content is
	// what we expect by comparing it to this value.
	sha, _ := hex.DecodeString("73867d92083f2f8ab899a26ccda7ef63dfaa0032a938620eda605558958a8041")
	// This is a deeply nested file that we use to verify that the content of
	// the tarball was loaded as expected.
	b, err := sandbox.ReadFile(fsys, "/usr/share/apk/keys/aarch64/alpine-devel@lists.alpinelinux.org-58199dcc.rsa.pub", 0)
	assert.OK(t, err)
	assert.Equal(t, len(b), 451)
	assert.Equal(t, sha256.Sum256(b), ([sha256.Size]byte)(sha))
	// Also verify that the metadata for the file is what we expected.
	s, err := sandbox.Stat(fsys, "/usr/share/apk/keys/aarch64/alpine-devel@lists.alpinelinux.org-58199dcc.rsa.pub")
	assert.OK(t, err)
	assert.Equal(t, s.Mode, 0444)
	assert.Equal(t, s.Size, 451)

	size, memsize, filesize := fsys.Size(), fsys.Memsize(), fsys.Filesize()
	t.Logf("Size     = %d\n", size)
	t.Logf("Memsize  = %d (%.2f%%)\n", memsize, 100*float64(memsize)/float64(size))
	t.Logf("Filesize = %d (%.2f%%)\n", filesize, 100*float64(filesize)/float64(size))
}
