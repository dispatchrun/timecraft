//go:build wasip1

package timecraft

import (
	"net/http"

	"github.com/stealthrocket/net/wasip1"
	"github.com/stealthrocket/timecraft/sdk"
)

// StartWorker starts a timecraft worker.
func StartWorker(handler http.Handler) error {
	client, err := NewClient()
	if err != nil {
		return err
	}
	logger := client.Logger()
	logger.Printf("starting worker")

	l, err := wasip1.Listen("tcp", sdk.WorkAddress)
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
