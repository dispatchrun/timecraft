package overlayfs_test

import (
	"io/fs"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
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

	specTests := []struct {
		scenario string
		function func(*testing.T, sandbox.FileSystem, sandbox.FileSystem)
	}{
		{
			scenario: "removed files that existed in the lower layer do no appear in the overlay file system",
			function: testOverlayFSUnlinkFile,
		},

		{
			scenario: "removed files that existed in a sub directory of the lower layer do no appear in the overlay file system",
			function: testOverlayFSUnlinkFileInDirectory,
		},

		{
			scenario: "recreating a file at a location that was removed from the lower layer and removing it does not cause the lower file to reappear",
			function: testOverlayFSUnlinkAndCreateAndUnlink,
		},

		{
			scenario: "unlinking a directory that exists in the lower layer errors with EISDIR",
			function: testOverlayFSUnlinkDirectory,
		},

		{
			scenario: "a file can be create in place where one existed in the lower layer",
			function: testOverlayFSCreateOverwrite,
		},

		{
			scenario: "a file can be create in place where one was removed in the lower layer",
			function: testOverlayFSCreateAfterUnlink,
		},

		{
			scenario: "creating a file with O_EXCL at a location that exists on the lower layer errors with EEXIST",
			function: testOverlayFSCreateExclusiveExists,
		},
	}

	for _, test := range specTests {
		t.Run(test.scenario, func(t *testing.T) {
			test.function(t,
				sandbox.DirFS(t.TempDir()),
				sandbox.DirFS(t.TempDir()),
			)
		})
	}
}

func testOverlayFSUnlinkFile(t *testing.T, lower, upper sandbox.FileSystem) {
	foo, err := sandbox.Create(lower, "foo", 0644)
	assert.OK(t, err)
	assert.OK(t, foo.Close())

	bar, err := sandbox.Create(lower, "bar", 0644)
	assert.OK(t, err)
	assert.OK(t, bar.Close())

	fsys := overlayfs.New(lower, upper)
	assert.OK(t, sandbox.Unlink(fsys, "foo"))

	_, err = sandbox.Open(fsys, "foo")
	assert.Error(t, err, sandbox.ENOENT)

	_, err = sandbox.Stat(fsys, "foo")
	assert.Error(t, err, sandbox.ENOENT)

	_, err = sandbox.Stat(fsys, "bar")
	assert.OK(t, err)
}

func testOverlayFSUnlinkFileInDirectory(t *testing.T, lower, upper sandbox.FileSystem) {
	err := sandbox.MkdirAll(lower, "path/to", 0755)
	assert.OK(t, err)

	foo, err := sandbox.Create(lower, "path/to/foo", 0644)
	assert.OK(t, err)
	assert.OK(t, foo.Close())

	bar, err := sandbox.Create(lower, "path/to/bar", 0644)
	assert.OK(t, err)
	assert.OK(t, bar.Close())

	fsys := overlayfs.New(lower, upper)
	assert.OK(t, sandbox.Unlink(fsys, "path/to/foo"))

	_, err = sandbox.Open(fsys, "path/to/foo")
	assert.Error(t, err, sandbox.ENOENT)

	_, err = sandbox.Stat(fsys, "path/to/foo")
	assert.Error(t, err, sandbox.ENOENT)

	_, err = sandbox.Stat(fsys, "path/to/bar")
	assert.OK(t, err)
}

func testOverlayFSUnlinkAndCreateAndUnlink(t *testing.T, lower, upper sandbox.FileSystem) {
	foo, err := sandbox.Create(lower, "foo", 0644)
	assert.OK(t, err)
	assert.OK(t, foo.Close())

	fsys := overlayfs.New(lower, upper)
	assert.OK(t, sandbox.Unlink(fsys, "foo"))

	f, err := sandbox.Create(fsys, "foo", 0644)
	assert.OK(t, err)
	assert.OK(t, f.Close())
	assert.OK(t, sandbox.Unlink(fsys, "foo"))

	_, err = sandbox.Open(fsys, "foo")
	assert.Error(t, err, sandbox.ENOENT)

	_, err = sandbox.Stat(fsys, "foo")
	assert.Error(t, err, sandbox.ENOENT)
}

func testOverlayFSUnlinkDirectory(t *testing.T, lower, upper sandbox.FileSystem) {
	err := sandbox.Mkdir(lower, "test", 0755)
	assert.OK(t, err)

	fsys := overlayfs.New(lower, upper)

	err = sandbox.Unlink(fsys, "test")
	assert.Error(t, err, sandbox.EISDIR)
}

func testOverlayFSCreateOverwrite(t *testing.T, lower, upper sandbox.FileSystem) {
	err := sandbox.WriteFile(lower, "test", []byte("foo"), 0644)
	assert.OK(t, err)

	fsys := overlayfs.New(lower, upper)

	err = sandbox.Unlink(fsys, "test")
	assert.OK(t, err)

	err = sandbox.WriteFile(fsys, "test", []byte("bar"), 0644)
	assert.OK(t, err)

	b, err := sandbox.ReadFile(fsys, "test", 0)
	assert.OK(t, err)
	assert.Equal(t, string(b), "bar")
}

func testOverlayFSCreateAfterUnlink(t *testing.T, lower, upper sandbox.FileSystem) {
	err := sandbox.WriteFile(lower, "test", []byte("foo"), 0644)
	assert.OK(t, err)

	fsys := overlayfs.New(lower, upper)

	err = sandbox.WriteFile(fsys, "test", []byte("bar"), 0644)
	assert.OK(t, err)

	b, err := sandbox.ReadFile(fsys, "test", 0)
	assert.OK(t, err)
	assert.Equal(t, string(b), "bar")
}

func testOverlayFSCreateExclusiveExists(t *testing.T, lower, upper sandbox.FileSystem) {
	err := sandbox.WriteFile(lower, "test", []byte("foo"), 0644)
	assert.OK(t, err)

	fsys := overlayfs.New(lower, upper)

	_, err = fsys.Open("test", sandbox.O_CREAT|sandbox.O_EXCL|sandbox.O_RDWR, 0644)
	assert.Error(t, err, sandbox.EEXIST)
}
