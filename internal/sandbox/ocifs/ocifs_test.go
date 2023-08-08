package ocifs_test

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/ocifs"
	"github.com/stealthrocket/timecraft/internal/sandbox/sandboxtest"
)

func TestOciFS(t *testing.T) {
	tests := []struct {
		scenario string
		makeFS   func(*testing.T, string) sandbox.FileSystem
	}{
		{
			scenario: "single layer",
			makeFS: func(t *testing.T, path string) sandbox.FileSystem {
				return ocifs.New(sandbox.DirFS(path))
			},
		},

		{
			scenario: "two identical layers",
			makeFS: func(t *testing.T, path string) sandbox.FileSystem {
				return ocifs.New(sandbox.DirFS(path), sandbox.DirFS(path))
			},
		},

		{
			scenario: "one empty layer below",
			makeFS: func(t *testing.T, path string) sandbox.FileSystem {
				return ocifs.New(sandbox.DirFS(t.TempDir()), sandbox.DirFS(path))
			},
		},

		{
			scenario: "one empty layer above",
			makeFS: func(t *testing.T, path string) sandbox.FileSystem {
				return ocifs.New(sandbox.DirFS(path), sandbox.DirFS(t.TempDir()))
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

			sandboxtest.TestRootFS(t, test.makeFS)
		})
	}
}

func TestOciFSLAyers(t *testing.T) {
	tests := []struct {
		scenario string
		files    []string
	}{
		{
			scenario: "whiteout file masks lower layer",
			files:    []string{"answer"},
		},

		{
			scenario: "whiteout directory masks lower layer",
			files:    []string{"answer"},
		},

		{
			scenario: "whiteout symlink masks lower layer",
			files:    []string{"answer"},
		},

		{
			scenario: "opaque whiteout masks all files in directory",
			files:    []string{"answer", "tmp/question"},
		},

		{
			scenario: "open file in lower layer",
			files:    []string{"home/answer", "home/message"},
		},

		{
			scenario: "merge directory trees",
			files: []string{
				"a",
				"b",
				"c",
				"a/b",
				"a/c",
				"a/b/a",
				"a/b/b",
				"a/b/c",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			paths, err := filepath.Glob(filepath.Join("testdata", strings.ReplaceAll(test.scenario, " ", "_"), "layer-*"))
			assert.OK(t, err)
			sort.Strings(paths)

			layers := make([]sandbox.FileSystem, len(paths))
			for i, path := range paths {
				layers[i] = sandbox.DirFS(path)
			}

			err = fstest.TestFS(sandbox.FS(ocifs.New(layers...)), test.files...)
			assert.OK(t, err)
		})
	}
}
