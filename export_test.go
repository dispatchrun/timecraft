package main_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

var export = tests{
	"show the export command help with the short option": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "export", "-h")
		assert.OK(t, err)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft export ")
		assert.Equal(t, stderr, "")
	},

	"show the export command help with the long option": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "export", "--help")
		assert.OK(t, err)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft export ")
		assert.Equal(t, stderr, "")
	},

	"export without a resource type": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "export")
		assert.ExitError(t, err, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "Expected resource type, id, and output file as argument")
	},

	"export without a resource id": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "export", "profile")
		assert.ExitError(t, err, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "Expected resource type, id, and output file as argument")
	},

	"export without an output file": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "export", "profile", "74080192e42e")
		assert.ExitError(t, err, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "Expected resource type, id, and output file as argument")
	},

	"export a module to stdout": func(t *testing.T) {
		stdout, processID, err := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.OK(t, err)
		assert.Equal(t, stdout, "sleeping for 1ns\n")
		assert.NotEqual(t, processID, "")

		moduleID, stderr, err := timecraft(t, "get", "mod", "-q")
		assert.OK(t, err)
		assert.Equal(t, stderr, "")
		moduleID = strings.TrimSuffix(moduleID, "\n")

		moduleData, stderr, err := timecraft(t, "export", "mod", moduleID, "-")
		assert.OK(t, err)
		assert.Equal(t, stderr, "")

		sleepWasm, err := os.ReadFile("./testdata/go/sleep.wasm")
		assert.OK(t, err)
		assert.True(t, moduleData == string(sleepWasm))
	},
}
