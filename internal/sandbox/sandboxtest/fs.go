package sandboxtest

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"golang.org/x/exp/maps"
)

// TestFS is a test suite based on testing/fstest using test data located in the
// directory path given as argument to the makeFS function.
//
// The test suite invokes makeFS to construct a fs.FS instance which is expected
// to expose the content of the given directory path.
func TestFS(t *testing.T, makeFS func(*testing.T, string) fs.FS) {
	path := testdata("fstest")
	fsys := makeFS(t, path)
	assert.OK(t, fstest.TestFS(fsys,
		"answer",
		"empty",
		"message",
		"tmp/one",
		"tmp/two",
		"tmp/three",
	))
}

// TestFileSystem is a test suite which validates the behavior of FileSystem
// implementations.
//
// The test suite invokes makeFS to constructs an empty file system that will
// be used to exercise the test scenarios.
func TestFileSystem(t *testing.T, makeFS func(*testing.T) sandbox.FileSystem) {
	t.Run("Create", func(t *testing.T) { fsTestCreate.run(t, makeFS) })
	t.Run("Open", func(t *testing.T) { fsTestOpen.run(t, makeFS) })
	t.Run("Stat", func(t *testing.T) { fsTestStat.run(t, makeFS) })
	t.Run("Lstat", func(t *testing.T) { fsTestLstat.run(t, makeFS) })
	t.Run("Link", func(t *testing.T) { fsTestLink.run(t, makeFS) })
	t.Run("Unlink", func(t *testing.T) { fsTestUnlink.run(t, makeFS) })
	t.Run("Rename", func(t *testing.T) { fsTestRename.run(t, makeFS) })
	t.Run("Mkdir", func(t *testing.T) { fsTestMkdir.run(t, makeFS) })
	t.Run("Rmdir", func(t *testing.T) { fsTestRmdir.run(t, makeFS) })
}

type fsTestSuite map[string]func(*testing.T, sandbox.FileSystem)

func (tests fsTestSuite) run(t *testing.T, makeFS func(*testing.T) sandbox.FileSystem) {
	names := maps.Keys(tests)
	slices.Sort(names)

	for _, name := range names {
		t.Run(name, func(t *testing.T) { tests[name](t, makeFS(t)) })
	}
}

func testdata(path string) string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "testdata", path)
}
