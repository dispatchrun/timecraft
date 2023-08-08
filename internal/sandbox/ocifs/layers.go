package ocifs

import (
	"sync/atomic"

	"github.com/stealthrocket/timecraft/internal/sandbox"
)

// The fileLayers type is used to manage shared references to a set of file
// layers from open file instances.
//
// Each fileLayers instance owns a reference to its parent, and each file
// instance owns a reference to its layers. When the file is closed, it
// decrements its layers reference and when this reaches zero, the layers close
// their files and decrement their parent reference count.
//
// This mechanism is necessary to allow open parent directories. At each layer
// the number of files may decrease, because upper layers masking lower layers
// may cause lower layers to be evicted during path resolution.
//
// When resolving a path like "/hello/world/../you", the presence of ".."
// indicates that the algorithm must go back to the parent of "world", but
// simply opening the parent directories of the layers of "world" may result
// in missing some of the layers because the "hello" directory may merge more
// layers than the "world" directory.
//
// An alternative approach could be to implement a recursive path resolution
// algorithm which keeps track of the visible layers in each call frame;
// however, it only works when performing path resolution starting at the root
// directory. When looking up files starting at an open directory, we still
// need the ability to resolve ".." from that directory and have it point at
// all the layers visible in the parent.
//
// Keeping a reference to the parent layers solves this problem, and reference
// counting ensures that we keep all the necessary files open, and close them
// when we have the guarantee that they won't be needed anymore.
type fileLayers struct {
	refc   atomic.Uintptr
	parent *fileLayers
	files  []sandbox.File
}

func (l *fileLayers) ref() {
	l.refc.Add(1)
}

func (l *fileLayers) unref() {
	if l.refc.Add(^uintptr(0)) == 0 {
		closeFiles(l.files)
		unref(l.parent)
	}
}

func ref(l *fileLayers) {
	if l != nil {
		l.ref()
	}
}

func unref(l *fileLayers) {
	if l != nil {
		l.unref()
	}
}

func closeFiles(files []sandbox.File) {
	for _, f := range files {
		f.Close()
	}
}
