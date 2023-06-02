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
	os.Exit(Root(os.Args[1:]...))
}
