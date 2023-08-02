package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	_ "github.com/stealthrocket/net/http"
)

const defaultUrl = "http://httpbin.io/anything"

func main() {
	var url string
	switch len(os.Args) {
	case 1:
		url = defaultUrl
	case 2:
		url = os.Args[1]
	default:
		log.Fatalf("usage: %s [URL]", os.Args[0])
	}

	res, err := http.DefaultClient.Get(url)
	if err != nil {
		panic(err)
	}

	fmt.Println(res.Proto, res.StatusCode, res.Status)
	for k, vv := range res.Header {
		for _, v := range vv {
			fmt.Printf("%s: %s\n", k, v)
		}
	}
	fmt.Println("")
	if _, err := io.Copy(os.Stdout, res.Body); err != nil {
		log.Fatal(err)
	}
}
