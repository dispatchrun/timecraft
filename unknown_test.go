package main_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

var unknown = tests{
	"an error is reported when invoking an unknown command": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "whatever")
		assert.Equal(t, exitCode, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "timecraft whatever: unknown command\n")
	},
}
