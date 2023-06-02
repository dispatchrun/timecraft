package main_test

import (
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
}
