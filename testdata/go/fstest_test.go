package main

import (
	"flag"
	"os"
	"testing"
	"testing/fstest"
)

var (
	path string
	args []string
)

func TestMain(m *testing.M) {
	flag.StringVar(&path, "path", ".", "Path to the file system to run the test suite against")
	flag.Parse()
	args = flag.Args()
	os.Exit(m.Run())
}

func TestFS(t *testing.T) {
	if err := fstest.TestFS(os.DirFS(path), args...); err != nil {
		t.Error(err)
	}
}
