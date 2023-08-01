package sandbox_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/stealthrocket/timecraft/internal/assert"
)

func testFS(t *testing.T, fsys fs.FS) {
	assert.OK(t, fstest.TestFS(fsys,
		"answer",
		"empty",
		"message",
		"tmp/one",
		"tmp/two",
		"tmp/three",
	))
}
