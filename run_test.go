package main_test

import (
	"path/filepath"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

var run = tests{
	"show the run command help with the short option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "run", "-h")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft run ")
		assert.Equal(t, stderr, "")
	},

	"show the run command help with the long option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "run", "--help")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft run ")
		assert.Equal(t, stderr, "")
	},

	"running with a configuration file which does not exist uses the default location": func(t *testing.T) {
		t.Setenv("TIMECRAFTCONFIG", filepath.Join(t.TempDir(), "path", "to", "nowehere.yaml"))

		stdout, stderr, exitCode := timecraft(t, "run", "./testdata/go/sleep.wasm", "0")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "sleeping for 0s\n")
		assert.NotEqual(t, stderr, "")
	},
}
