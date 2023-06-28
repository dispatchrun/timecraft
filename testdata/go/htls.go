package main

import (
	"io"
	"log"
	"net/http"

	"github.com/stealthrocket/timecraft/sdk/go/timecraft"
)

func main() {
	client := &http.Client{Transport: &http.Transport{
		DialTLSContext: timecraft.DialTLS,
	}}

	for i := 0; i < 10; i++ {
		resp, err := client.Get("https://postman-echo.com/get")
		if err != nil {
			log.Fatalln(err)
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		log.Println("request", i, "done")
	}
}
