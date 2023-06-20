//go:build !linux

package timecraft

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

func makeSocketPath() (socketPath string, cleanup func() error) {
	socketPath = filepath.Join(os.TempDir(), fmt.Sprintf("timecraft.%s.sock", uuid.NewString()))

	cleanup = func() error {
		return os.Remove(socketPath)
	}

	return
}
