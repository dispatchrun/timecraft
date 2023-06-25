package main_test

import (
	"strings"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

var version = tests{
	"show the version command help with the short option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "version", "-h")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft version\n")
		assert.Equal(t, stderr, "")
	},

	"show the version command help with the long option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "version", "--help")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft version\n")
		assert.Equal(t, stderr, "")
	},

	"the version starts with the prefix timecraft": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "version")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "timecraft ")
		assert.Equal(t, stderr, "")
	},

	"the version number is not empty": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "version")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stderr, "")

		_, version, _ := strings.Cut(string(stdout), " ")
		assert.NotEqual(t, version, "")
	},

	"passing an unsupported flag to the command causes an error": func(t *testing.T) {
		_, _, exitCode := timecraft(t, "version", "-_")
		assert.Equal(t, exitCode, 2)
	},
}
