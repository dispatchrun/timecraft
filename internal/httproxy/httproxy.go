package httproxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

var mu sync.Mutex
var port = 6789

// Start an HTTP proxy. Cancel context to terminate.
func Start(ctx context.Context) {
	mu.Lock()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	port++
	mu.Unlock()

	s := &http.Server{
		Addr:           addr,
		Handler:        http.HandlerFunc(handler),
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}
	go func() {
		log.Printf("httproxy - starting server on %s", addr)
		err := s.ListenAndServe()
		if err == http.ErrServerClosed {
			log.Println("httproxy - server terminated")
		} else {
			log.Println("httproxy - server terminated with error", err)
		}
	}()
}

func handler(w http.ResponseWriter, req *http.Request) {
	u := *req.URL
	u.Scheme = "https"
	u.Host = req.Host
	outgoing, err := http.NewRequest(req.Method, u.String(), req.Body)
	if err != nil {
		log.Println("httproxy - invalid request:", err)
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	copyHeader(outgoing.Header, req.Header)

	client := http.DefaultClient

	resp, err := client.Do(outgoing)
	if err != nil {
		log.Println("httproxy - could not perform request:", err)
		http.Error(w, "could not perform request", http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)

	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Println("httproxy - could not write back body:", err)
	}
}

func copyHeader(to, from http.Header) {
	for k, vs := range from {
		for _, v := range vs {
			to.Add(k, v)
		}
	}
}
