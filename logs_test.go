package main_test

import (
	"strings"
	"testing"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
)

const text = `
Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.
Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.`

var logs = tests{
	"show the logs command help with the short option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "logs", "-h")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft logs ")
		assert.Equal(t, stderr, "")
	},

	"show the logs command help with the long option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "logs", "--help")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft logs ")
		assert.Equal(t, stderr, "")
	},

	"the output of a run is available when printing its logs": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "run", "./testdata/go/echo.wasm", "-n", text)
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, text[1:])
		processID := strings.TrimSpace(stderr)

		stdout, stderr, exitCode = timecraft(t, "logs", processID)
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, text[1:])
		assert.Equal(t, stderr, "")
	},
}
