//go:build linux

package timecraft

import (
	"fmt"
	"github.com/google/uuid"
)

func makeSocketPath() (socketPath string, cleanup func() error) {
	// Use abstract unix sockets on Linux.
	return fmt.Sprintf("@timecraft.%s.sock", uuid.NewString()), func() error { return nil }
}
