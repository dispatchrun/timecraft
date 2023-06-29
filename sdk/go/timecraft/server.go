//go:build wasip1

package timecraft

import (
	"net/http"

	"github.com/stealthrocket/net/wasip1"
)

// ListenAndServe is like the http.ListenAndServe function from the standard
// net/http package. It starts a timecraft worker accepting http requests on
// the given address.
func ListenAndServe(addr string, handler http.Handler) error {
	client, err := NewClient()
	if err != nil {
		return err
	}
	logger := client.Logger()
	logger.Printf("starting worker")

	l, err := wasip1.Listen("tcp", addr)
	if err != nil {
		return err
	}
	return http.Serve(l, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		taskID := r.Header.Get("X-Timecraft-Task")
		creator := r.Header.Get("X-Timecraft-Creator")

		logger.Printf("executing task %s for process %s", taskID, creator)

		handler.ServeHTTP(w, r)
	}))
}
