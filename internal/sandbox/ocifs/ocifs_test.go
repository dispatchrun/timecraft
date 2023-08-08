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
		present  fileMap
		missing  fileMap
	}{
		{
			scenario: "whiteout file masks lower layer",
			present:  fileMap{"answer": 0},
		},

		{
			scenario: "whiteout directory masks lower layer",
			present:  fileMap{"answer": 0},
		},

		{
			scenario: "whiteout symlink masks lower layer",
			present:  fileMap{"answer": 0},
		},

		{
			scenario: "whiteout masks file in sub directory",
			present: fileMap{
				"a":     fs.ModeDir,
				"a/a":   0,
				"a/b":   fs.ModeDir,
				"a/c":   0,
				"a/b/a": 0,
				"a/b/c": 0,
			},
			missing: fileMap{
				"a/b/b": 0,
			},
		},

		{
			scenario: "opaque whiteout masks all files in directory",
			present: fileMap{
				"answer":       0,
				"tmp":          fs.ModeDir,
				"tmp/question": 0,
			},
			missing: fileMap{
				"tmp/a": 0,
				"tmp/b": 0,
				"tmp/c": 0,
			},
		},

		{
			scenario: "open file in lower layer",
			present: fileMap{
				"home":         fs.ModeDir,
				"home/answer":  0,
				"home/message": 0,
			},
		},

		{
			scenario: "merge directory trees",
			present: fileMap{
				"a":     fs.ModeDir,
				"b":     fs.ModeDir,
				"c":     fs.ModeDir,
				"a/b":   fs.ModeDir,
				"a/c":   fs.ModeDir,
				"a/b/a": fs.ModeDir,
				"a/b/b": fs.ModeDir,
				"a/b/c": fs.ModeDir,
			},
		},

		{
			scenario: "directory masks file",
			present:  fileMap{"test": fs.ModeDir},
		},

		{
			scenario: "directory masks symlink masks file",
			present:  fileMap{"test": fs.ModeDir},
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

			files := make([]string, 0, len(test.present))
			for name, mode := range test.present {
				switch mode {
				case 0, fs.ModeDir:
					files = append(files, name)
				}
			}
			sort.Strings(files)

			fsys := ocifs.New(layers...)
			assert.OK(t, fstest.TestFS(sandbox.FS(fsys), files...))

			t.Run("present", func(t *testing.T) {
				for _, name := range test.present.names() {
					t.Run(name, func(t *testing.T) {
						mode := test.present[name]

						info, err := sandbox.Lstat(fsys, name)
						assert.OK(t, err)
						assert.Equal(t, info.Mode.Type(), mode)

						switch mode {
						case 0, fs.ModeDir:
							f, err := sandbox.Open(fsys, name)
							assert.OK(t, err)

							s, err := f.Stat("", 0)
							assert.OK(t, err)
							assert.OK(t, f.Close())

							assert.Equal(t, s.Mode.Type(), mode)
						}
					})
				}
			})

			t.Run("missing", func(t *testing.T) {
				for _, name := range test.missing.names() {
					t.Run(name, func(t *testing.T) {
						var err error

						_, err = sandbox.Lstat(fsys, name)
						assert.Error(t, err, sandbox.ENOENT)

						_, err = sandbox.Open(fsys, name)
						assert.Error(t, err, sandbox.ENOENT)
					})
				}
			})
		})
	}
}

type fileMap map[string]fs.FileMode

func (m fileMap) names() []string {
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	return names
}
