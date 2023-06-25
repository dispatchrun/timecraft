package main_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
)

var get = tests{
	"show the get command help with the short option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "get", "-h")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft get ")
		assert.Equal(t, stderr, "")
	},

	"show the get command help with the long option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "get", "--help")
		assert.Equal(t, exitCode, 0)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft get ")
		assert.Equal(t, stderr, "")
	},

	"get without a resource type": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "get")
		assert.Equal(t, exitCode, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "Expected at least the resource type as argument")
	},

	"get configs on an empty time machine": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "get", "configs")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "CONFIG ID  RUNTIME  MODULES  SIZE\n")
		assert.Equal(t, stderr, "")
	},

	"get modules on an empty time machine": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "get", "modules")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "MODULE ID  MODULE NAME  SIZE\n")
		assert.Equal(t, stderr, "")
	},

	"get process on an empty time machine": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "get", "processes")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "PROCESS ID  START  SIZE\n")
		assert.Equal(t, stderr, "")
	},

	"get runtimes on an empty time machine": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "get", "runtimes")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "RUNTIME ID  RUNTIME NAME  VERSION\n")
		assert.Equal(t, stderr, "")
	},

	"get config after run": func(t *testing.T) {
		stdout, processID, exitCode := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "sleeping for 1ns\n")
		assert.NotEqual(t, processID, "")

		configID, stderr, exitCode := timecraft(t, "get", "conf", "-q")
		assert.Equal(t, exitCode, 0)
		assert.NotEqual(t, configID, "")
		assert.Equal(t, stderr, "")
	},

	"get process after run": func(t *testing.T) {
		stdout, processID, exitCode := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "sleeping for 1ns\n")

		procID, stderr, exitCode := timecraft(t, "get", "proc", "-q")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, procID, processID)
		assert.Equal(t, stderr, "")
	},

	"get runtime after run": func(t *testing.T) {
		stdout, processID, exitCode := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.Equal(t, exitCode, 0)
		assert.Equal(t, stdout, "sleeping for 1ns\n")
		assert.NotEqual(t, processID, "")

		runtimeID, stderr, exitCode := timecraft(t, "get", "rt", "-q")
		assert.Equal(t, exitCode, 0)
		assert.NotEqual(t, runtimeID, "")
		assert.Equal(t, stderr, "")
	},
}
