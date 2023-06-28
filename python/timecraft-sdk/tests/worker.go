//go:build wasip1

package main

import (
	"fmt"
	"net/http"

	"github.com/stealthrocket/net/wasip1"
	"github.com/stealthrocket/timecraft/sdk"
)

func main() {
	l, err := wasip1.Listen("unix", sdk.WorkSocket)
	if err != nil {
		panic(err)
	}

	err = http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Println(req.Method, req.URL)
		w.WriteHeader(200)
		w.Write([]byte("OK!"))
	}))
	if err != nil {
		panic(err)
	}
}
