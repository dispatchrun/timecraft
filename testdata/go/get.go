package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	_ "github.com/stealthrocket/timecraft/sdk/go/timecraft"
)

func main() {
	flag.Parse()

	stdout := bufio.NewWriter(os.Stdout)
	defer stdout.Flush()

	for _, arg := range flag.Args() {
		r, err := http.Get(arg)
		if err != nil {
			log.Fatal(err)
		}
		io.Copy(stdout, r.Body)
		r.Body.Close()
		stdout.WriteByte('\n')
	}
}
