package main_test

import (
	"fmt"
	"os"
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

	"run the testing/fstest test suite": func(t *testing.T) {
		testRun(t, "testdata/go/fstest_test.wasm", "-path=testdata/fs",
			// This is the list of files expected to be found in the test directory.
			"empty",
			"message",
			"tmp/file-1",
			"tmp/file-2",
		)
	},

	"run the x/net/nettest test suite": func(t *testing.T) {
		testRun(t, "testdata/go/nettest_test.wasm")
	},
}

func testRun(t *testing.T, module string, args ...string) {
	command := append([]string{"run", "--", module}, args...)
	stdout, stderr, exitCode := timecraft(t, command...)
	if exitCode != 0 {
		fmt.Fprintf(os.Stdout, "=== STDOUT:\n%s", stdout)
		fmt.Fprintf(os.Stderr, "=== STDERR:\n%s", stderr)
	}
	assert.Equal(t, exitCode, 0)
}
