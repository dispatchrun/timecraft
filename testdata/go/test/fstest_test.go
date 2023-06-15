package main

import (
	"os"
	"testing"
	"testing/fstest"
)

func TestFS(t *testing.T) {
	// TODO: fix the current working directory in timecraft tests
	if err := fstest.TestFS(os.DirFS("testdata/fs"),
		"empty",
		"message",
		"tmp/file-1",
		"tmp/file-2",
	); err != nil {
		t.Error(err)
	}
}
