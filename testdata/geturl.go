package main

import (
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
	io.Copy(os.Stdout, r.Body)
}
