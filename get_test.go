package main_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

var get = tests{
	"show the get command help with the short option": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "get", "-h")
		assert.OK(t, err)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft get ")
		assert.Equal(t, stderr, "")
	},

	"show the get command help with the long option": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "get", "--help")
		assert.OK(t, err)
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft get ")
		assert.Equal(t, stderr, "")
	},

	"get without a resource type": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "get")
		assert.ExitError(t, err, 2)
		assert.Equal(t, stdout, "")
		assert.HasPrefix(t, stderr, "Expected exactly one resource type as argument")
	},

	"get configs on an empty time machine": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "get", "configs")
		assert.OK(t, err)
		assert.Equal(t, stdout, "CONFIG ID  RUNTIME  MODULES  SIZE\n")
		assert.Equal(t, stderr, "")
	},

	"get logs on an empty time machine": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "get", "logs")
		assert.OK(t, err)
		assert.Equal(t, stdout, "PROCESS ID  SEGMENTS  START  SIZE\n")
		assert.Equal(t, stderr, "")
	},

	"get modules on an empty time machine": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "get", "modules")
		assert.OK(t, err)
		assert.Equal(t, stdout, "MODULE ID  MODULE NAME  SIZE\n")
		assert.Equal(t, stderr, "")
	},

	"get process on an empty time machine": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "get", "processes")
		assert.OK(t, err)
		assert.Equal(t, stdout, "PROCESS ID  START\n")
		assert.Equal(t, stderr, "")
	},

	"get runtimes on an empty time machine": func(t *testing.T) {
		stdout, stderr, err := timecraft(t, "get", "runtimes")
		assert.OK(t, err)
		assert.Equal(t, stdout, "RUNTIME ID  RUNTIME NAME  VERSION\n")
		assert.Equal(t, stderr, "")
	},

	"get config after run": func(t *testing.T) {
		stdout, processID, err := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.OK(t, err)
		assert.Equal(t, stdout, "sleeping for 1ns\n")
		assert.NotEqual(t, processID, "")

		configID, stderr, err := timecraft(t, "get", "conf", "-q")
		assert.OK(t, err)
		assert.NotEqual(t, configID, "")
		assert.Equal(t, stderr, "")
	},

	"get log after run": func(t *testing.T) {
		stdout, processID, err := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.OK(t, err)
		assert.Equal(t, stdout, "sleeping for 1ns\n")

		logID, stderr, err := timecraft(t, "get", "log", "-q")
		assert.OK(t, err)
		assert.Equal(t, logID, processID)
		assert.Equal(t, stderr, "")
	},

	"get process after run": func(t *testing.T) {
		stdout, processID, err := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.OK(t, err)
		assert.Equal(t, stdout, "sleeping for 1ns\n")

		procID, stderr, err := timecraft(t, "get", "proc", "-q")
		assert.OK(t, err)
		assert.Equal(t, procID, processID)
		assert.Equal(t, stderr, "")
	},

	"get runtime after run": func(t *testing.T) {
		stdout, processID, err := timecraft(t, "run", "./testdata/go/sleep.wasm", "1ns")
		assert.OK(t, err)
		assert.Equal(t, stdout, "sleeping for 1ns\n")
		assert.NotEqual(t, processID, "")

		runtimeID, stderr, err := timecraft(t, "get", "rt", "-q")
		assert.OK(t, err)
		assert.NotEqual(t, runtimeID, "")
		assert.Equal(t, stderr, "")
	},
}
