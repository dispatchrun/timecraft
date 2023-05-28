package main

import (
	"fmt"
	"os"
	"time"
)

func main() {
	durations := make([]time.Duration, len(os.Args)-1)

	for i, sleep := range os.Args[1:] {
		d, err := time.ParseDuration(sleep)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERR: %s: %s", sleep, err)
			os.Exit(1)
		}
		durations[i] = d
	}

	for _, sleep := range durations {
		fmt.Printf("sleeping for %s\n", sleep)
		time.Sleep(sleep)
	}
}
