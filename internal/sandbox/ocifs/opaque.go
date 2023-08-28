package ocifs

import (
	"bytes"
	"sync"

	"github.com/stealthrocket/timecraft/internal/sandbox"
	"github.com/stealthrocket/timecraft/internal/sandbox/tarfs"
)

//go:embedded opaque.tar
var opaque []byte

var (
	opaqueOnce sync.Once
	opaqueFS   *tarfs.FileSystem
)

// Opaque returns a read-only file system which contains a singel .wh..wh..opq
// file at its root.
//
// This file system is useful as a mask to hide all files of lower layers of a
// OCI file system.
func Opaque() sandbox.FileSystem {
	opaqueOnce.Do(func() {
		opaqueFS, _ = tarfs.Open(bytes.NewReader(opaque), int64(len(opaque)))
	})
	return opaqueFS
}
