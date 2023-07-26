package main_test

import (
	"testing"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/stealthrocket/timecraft/internal/assert"
	config "github.com/stealthrocket/timecraft/internal/timecraft"
)

var pull = tests{
	"pull public debian image": func(t *testing.T) {
		_, _, exitCode := timecraft(t, "pull", "debian")
		assert.Equal(t, exitCode, 0)

		path, _ := config.ImagesPath.Resolve()

		p, err := layout.FromPath(path)
		if err != nil {
			t.Error(err)
		}
		index, err := p.ImageIndex()
		if err != nil {
			t.Error(err)
		}
		manifests, err := index.IndexManifest()
		if err != nil {
			t.Error(err)
		}

		var found bool
		for _, desc := range manifests.Manifests {
			if name, ok := desc.Annotations["name"]; ok {
				if name == "debian" {
					found = true
					break
				}
			}
		}
		if !found {
			t.Error("debian image not found")
		}
	},
}
