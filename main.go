package main

import (
	"context"
	"os"

	"github.com/stealthrocket/timecraft/internal/cmd"
)

func main() {
	os.Exit(cmd.Root(context.Background(), os.Args[1:]...))
}
