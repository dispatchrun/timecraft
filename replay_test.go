package main_test

import (
	"strings"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

var replay = tests{
	"show the replay command help with the short option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "replay", "-h")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft replay ")
		assert.Equal(t, stderr, "")
	},

	"show the replay command help with the long option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "replay", "--help")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft replay ")
		assert.Equal(t, stderr, "")
	},

	"standard output is printed during replays": func(t *testing.T) {
		stdout, processID, exitCode := timecraft(t, "run", "--", "./testdata/go/urandom.wasm")
		assert.Equal(t, exitCode, 0)

		replay, stderr, exitCode := timecraft(t, "replay", strings.TrimSpace(processID))
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, replay, stdout)
		assert.Equal(t, stderr, "")
	},

	"standard output is not printed during quiet replays": func(t *testing.T) {
		stdout, processID, exitCode := timecraft(t, "run", "--", "./testdata/go/urandom.wasm")
		assert.Equal(t, exitCode, 0)
		assert.NotEqual(t, stdout, "")

		replay, stderr, exitCode := timecraft(t, "replay", strings.TrimSpace(processID), "-q")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, replay, "")
		assert.Equal(t, stderr, "")
	},
}
