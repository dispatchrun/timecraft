package main_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
)

var export = tests{
	"show the export command help with the short option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "export", "-h")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft export ")
		assert.Equal(t, stderr, "")
	},

	"show the export command help with the long option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "export", "--help")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft export ")
		assert.Equal(t, stderr, "")
	},

	"export without a resource type": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "export")
		assert.Equal(t, exitCode, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "Expected resource type, id, and output file as argument")
	},

	"export without a resource id": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "export", "profile")
		assert.Equal(t, exitCode, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "Expected resource type, id, and output file as argument")
	},

	"export without an output file": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "export", "profile", "74080192e42e")
		assert.Equal(t, exitCode, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "Expected resource type, id, and output file as argument")
	},

	"export a module to stdout": func(t *testing.T) {
		stdout, processID, exitCode := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "sleeping for 1ns\n")
		assert.NotEqual(t, processID, "")

		moduleID, stderr, exitCode := timecraft(t, "get", "mod", "-q")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stderr, "")
		moduleID = strings.TrimSuffix(moduleID, "\n")

		moduleData, stderr, exitCode := timecraft(t, "export", "mod", moduleID, "-")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stderr, "")

		sleepWasm, err := os.ReadFile("./testdata/go/sleep.wasm")
		assert.OK(t, err)
		assert.True(t, moduleData == string(sleepWasm))
	},
}
