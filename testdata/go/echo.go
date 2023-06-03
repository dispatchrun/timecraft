package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

func main() {
	var n bool
	flag.BoolVar(&n, "n", false, "do not print a new line")
	flag.Parse()

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	args := flag.Args()
	if len(args) > 0 {
		fmt.Fprintf(w, "%s", args[0])

		for _, arg := range args[1:] {
			fmt.Fprintf(w, " %s", arg)
		}
	}

	if !n {
		fmt.Fprintln(w)
	}
}
