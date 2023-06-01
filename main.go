package main

import (
	"context"
	"io"
	"log"
	"os"
)

func init() {
	// TODO: do something better with logs
	log.SetOutput(io.Discard)
}

func main() {
	os.Exit(Root(context.Background(), os.Args[1:]...))
}
