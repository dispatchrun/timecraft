package main_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

var run = tests{
	"show the run command help with the short option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "run", "-h")
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft run ")
		assert.Equal(t, stderr, "")
		assert.Equal(t, exitCode, 0)
	},

	"show the run command help with the long option": func(t *testing.T) {
		stdout, stderr, exitCode := timecraft(t, "run", "--help")
		assert.HasPrefix(t, stdout, "Usage:\ttimecraft run ")
		assert.Equal(t, stderr, "")
		assert.Equal(t, exitCode, 0)
	},

	"run Go tests": func(t *testing.T) {
		files, _ := filepath.Glob("testdata/go/test/*_test.wasm")
		if len(files) == 0 {
			t.Fatal("there are no Go tests to run, this does not seem right")
		}
		for _, file := range files {
			t.Run(file, func(t *testing.T) { testRun(t, file) })
		}
	},
}

func testRun(t *testing.T, module string, args ...string) {
	command := append([]string{"run", "--trace", "--", module, "-test.v"}, args...)

	stdout, stderr, exitCode := timecraft(t, command...)
	if exitCode != 0 {
		fmt.Fprintf(os.Stdout, "STDOUT:\n%s", stdout)
		fmt.Fprintf(os.Stderr, "STDERR:\n%s", stderr)
	}

	assert.Equal(t, exitCode, 0)
}
