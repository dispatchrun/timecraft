package sandbox_test

import (
	"io/fs"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox"
)

func TestSubFS(t *testing.T) {
	base := sandbox.DirFS(t.TempDir())

	err := sandbox.MkdirAll(base, "/tmp/path", 0711)
	assert.OK(t, err)

	err = sandbox.WriteFile(base, "/foo", []byte("hello"), 0644)
	assert.OK(t, err)

	err = sandbox.WriteFile(base, "/tmp/bar", []byte("world"), 0600)
	assert.OK(t, err)

	fsys := sandbox.SubFS(base, "/test/sub")

	b1, err := sandbox.ReadFile(fsys, "/test/sub/foo", 0)
	assert.OK(t, err)
	assert.Equal(t, string(b1), "hello")

	b2, err := sandbox.ReadFile(fsys, "/test/sub/tmp/bar", 0)
	assert.OK(t, err)
	assert.Equal(t, string(b2), "world")

	s1, err := sandbox.Stat(fsys, "/./../test/./../test/sub/./../sub/tmp/./../../sub/tmp/path")
	assert.OK(t, err)
	assert.Equal(t, s1.Mode, fs.ModeDir|0711)

	s2, err := sandbox.Stat(fsys, "/test/sub/foo")
	assert.OK(t, err)
	assert.Equal(t, s2.Mode, 0644)

	s3, err := sandbox.Stat(fsys, "/test/sub/tmp/bar")
	assert.OK(t, err)
	assert.Equal(t, s3.Mode, 0600)
}
