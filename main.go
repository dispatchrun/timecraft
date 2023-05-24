package main

import (
	"fmt"
	"os"

	"github.com/stealthrocket/timecraft/internal/cmd"
)

func main() {
	if err := cmd.Root(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
