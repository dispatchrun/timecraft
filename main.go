package main

import (
	"io"
	"log"
	"os"
)

func init() {
	// TODO: do something better with logs
	log.SetOutput(io.Discard)
}

func main() {
	os.Exit(root(os.Args[1:]...))
}
