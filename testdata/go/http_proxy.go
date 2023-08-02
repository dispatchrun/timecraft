package main

import (
	"flag"
	"log"
	"net/http"
	"net/http/httputil"

	_ "github.com/stealthrocket/net/http"
	"github.com/stealthrocket/net/wasip1"
)

func main() {
	var addr string
	flag.StringVar(&addr, "http", ":80", "Network address to listen on for incoming connections")
	flag.Parse()

	log.Printf("listening on %s", addr)
	l, err := wasip1.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()

	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.Out.URL.Scheme = "http"
			r.Out.URL.Host = r.In.Host
			r.Out.Header.Add("X-Injected-Request-Header", "foo")
		},
		ModifyResponse: func(r *http.Response) error {
			r.Header.Add("X-Injected-Response-Header", "bar")
			return nil
		},
	}

	if err := http.Serve(l, proxy); err != nil {
		log.Fatal(err)
	}
}
