package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	_ "github.com/stealthrocket/timecraft/sdk/go/wasi_experimental_http"
)

func main() {
	r, err := http.Get(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer r.Body.Close()
	fmt.Fprintf(os.Stderr, "%s %d %s\r\n", r.Proto, r.StatusCode, r.Status)
	r.Header.Write(os.Stderr)
	fmt.Fprintln(os.Stderr)
	io.CopyBuffer(os.Stdout, r.Body, make([]byte, 1024))
}
