package main

import (
	"context"
	"os"
)

func main() {
	os.Exit(Root(context.Background(), os.Args[1:]...))
}
