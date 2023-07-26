package timecraft

import (
	"fmt"

	"github.com/containers/storage/pkg/archive"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/layout"
	"github.com/google/go-containerregistry/pkg/v1/match"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type Images struct {
	imageRegistryPath string
}

func NewImages(config *Config) (*Images, error) {
	imagesPath, ok := config.Images.Location.Value()
	if !ok {
		return nil, fmt.Errorf("images path not set")
	}
	path, err := imagesPath.Resolve()
	if err != nil {
		return nil, fmt.Errorf("images path invalid: %w", err)
	}
	if err := createDirectory(path); err != nil {
		return nil, err
	}
	return &Images{
		imageRegistryPath: path,
	}, nil
}

func (i *Images) Unpack(name string, dst string) error {
	img, err := i.image(name)
	if err != nil {
		return err
	}

	layers, err := img.Layers()
	if err != nil {
		return err
	}

	for _, layer := range layers {
		reader, err := layer.Compressed()
		if err != nil {
			return err
		}
		if err := archive.Untar(reader, dst, nil); err != nil {
			return err
		}
	}

	return nil
}

func (i *Images) image(name string) (v1.Image, error) {
	p, err := layout.FromPath(i.imageRegistryPath)
	if err != nil {
		return nil, err
	}
	index, err := p.ImageIndex()
	if err != nil {
		return nil, err
	}
	manifests, err := index.IndexManifest()
	if err != nil {
		return nil, err
	}

	var h v1.Hash
	for _, desc := range manifests.Manifests {
		if name, ok := desc.Annotations["name"]; ok {
			if name == name {
				h = desc.Digest
				break
			}
		}
	}

	return p.Image(h)
}

func (i *Images) Pull(n string) error {
	ref, err := name.ParseReference(n)
	if err != nil {
		return err
	}

	opts := []layout.Option{}
	opts = append(opts, layout.WithAnnotations(map[string]string{
		"org.opencontainers.image.ref.name": ref.Name(),
		"name":                              n,
	}))

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return err
	}

	p, err := layout.FromPath(i.imageRegistryPath)
	if err != nil {
		p, err = layout.Write(i.imageRegistryPath, empty.Index)
		if err != nil {
			return err
		}
	}

	return p.ReplaceImage(
		img,
		match.Annotation("org.opencontainers.image.ref.name", ref.Name()),
		opts...,
	)
}
