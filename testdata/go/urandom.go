package main

import (
	"fmt"
	"io"
	"log"
	"os"
)

func main() {
	f, err := os.Open("/dev/urandom")
	if err != nil {
		log.Fatalf("cannot open random device: %w", err)
	}
	defer f.Close()

	b := make([]byte, 32)
	if _, err := io.ReadFull(f, b); err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%x\n", b)
}
