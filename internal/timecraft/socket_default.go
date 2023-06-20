//go:build !linux

package timecraft

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

func makeSocketPath() (socketPath string, cleanup func()) {
	socketPath = filepath.Join(os.TempDir(), fmt.Sprintf("timecraft.%s.sock", uuid.NewString()))
	cleanup = func() { os.Remove(socketPath) }
	return
}
