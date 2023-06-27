package main

import (
	"flag"
	"log"
	"net/http"
	"path/filepath"

	"github.com/stealthrocket/net/wasip1"
)

var (
	bind string
)

func main() {
	flag.StringVar(&bind, "http", ":3000", "Network address to listen on for incoming connections")
	flag.Parse()

	path := "."
	args := flag.Args()
	switch len(args) {
	case 0:
	case 1:
		path = args[0]
	default:
		log.Fatalf("too many paths to serve files from: %q", args)
	}

	path, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}

	l, err := wasip1.Listen("tcp", bind)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	server := &http.Server{
		Handler: http.FileServer(http.Dir(path)),
	}
	defer server.Close()

	log.Printf("%s: serving files on %s", path, bind)
	if err := server.Serve(l); err != nil {
		if err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}
}
