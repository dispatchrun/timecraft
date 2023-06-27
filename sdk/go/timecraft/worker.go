//go:build wasip1

package timecraft

import (
	"net/http"

	"github.com/stealthrocket/net/wasip1"
	"github.com/stealthrocket/timecraft/sdk"
)

// StartWorker starts a timecraft worker.
func StartWorker(handler http.Handler) error {
	l, err := wasip1.Listen("tcp", sdk.WorkSocket)
	if err != nil {
		return err
	}
	return http.Serve(l, handler)
}
