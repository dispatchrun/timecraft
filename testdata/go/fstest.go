package main

import (
	"flag"
	"log"
	"os"
	"testing/fstest"
)

func main() {
	var path string
	flag.StringVar(&path, "path", ".", "Path to the file system to run the test suite against")
	flag.Parse()

	if err := fstest.TestFS(os.DirFS(path), flag.Args()...); err != nil {
		log.Fatal(err)
	}
}
