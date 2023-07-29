package sandboxtest

import (
	"maps"
	"slices"
	"testing"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

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
