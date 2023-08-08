package container

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
)

// ImageCache is an implementation of oras.ReadOnlyRegistry which copies remote
// images fetched by the program to a local store for faster retrieval.
//
// The cache uses the "org.opencontainers.image.ref.name" annotation to perform
// automatic name resolution on the images.
//
// ImageCache values are safe for concurrent use by multiple goroutines as long
// as none of its properties are mutated while its methods are called.
type ImageCache struct {
	// The local store where images and tags are cached.
	//
	// If nil, the cache does not store images but still performs resolution of
	// the image names.
	LocalStore oras.Target
	// If non-nil, configures a platform of images that can be fetched.
	TargetPlatform *ocispec.Platform
	// A function used to construct the remote repository used to lookupg remote
	// images.
	//
	// This function can usually be left nil, the cache will create instances of
	// remote.Repository by default. It is useful to set if the configuration of
	// the remote.Repository instance needs to be customized.
	RemoteRepository func(registry.Reference) oras.ReadOnlyTarget
}

func (c *ImageCache) Fetch(ctx context.Context, target ocispec.Descriptor) (io.ReadCloser, error) {
	artifact, ok := target.Annotations[ocispec.AnnotationRefName]
	if !ok {
		if c.LocalStore != nil {
			return c.LocalStore.Fetch(ctx, target)
		}
		return nil, fmt.Errorf("%s: fetch: %w", target.Digest, errdef.ErrMissingReference)
	}

	ref, err := registry.ParseReference(artifact)
	if err != nil {
		return nil, err
	}

	if c.LocalStore == nil {
		return c.repository(ref).Fetch(ctx, target)
	}

	for {
		r, err := c.LocalStore.Fetch(ctx, target)
		if !errors.Is(err, errdef.ErrNotFound) {
			return r, err
		}

		options := oras.DefaultCopyOptions
		options.WithTargetPlatform(c.TargetPlatform)

		target, err = oras.Copy(ctx, c.repository(ref), ref.Reference, c.LocalStore, artifact, options)
		if err != nil {
			return nil, err
		}
	}
}

func (c *ImageCache) Exists(ctx context.Context, target ocispec.Descriptor) (bool, error) {
	if c.LocalStore != nil {
		if exists, err := c.LocalStore.Exists(ctx, target); exists || err != nil {
			return exists, err
		}
	}
	artifact, ok := target.Annotations[ocispec.AnnotationRefName]
	if !ok {
		return false, nil
	}
	ref, err := registry.ParseReference(artifact)
	if err != nil {
		return false, err
	}
	return c.repository(ref).Exists(ctx, target)
}

func (c *ImageCache) Resolve(ctx context.Context, reference string) (ocispec.Descriptor, error) {
	if c.LocalStore != nil {
		desc, err := c.LocalStore.Resolve(ctx, reference)
		if !errors.Is(err, errdef.ErrNotFound) {
			return desc, err
		}
	}

	ref, err := registry.ParseReference(reference)
	if err != nil {
		return ocispec.Descriptor{}, fmt.Errorf("%s: resolve: %w", reference, errdef.ErrNotFound)
	}

	desc, err := oras.Resolve(ctx, c.repository(ref), ref.Reference, oras.ResolveOptions{
		TargetPlatform: c.TargetPlatform,
	})
	if err != nil {
		return ocispec.Descriptor{}, err
	}
	if desc.Annotations == nil {
		desc.Annotations = make(map[string]string)
	}
	reference = ref.String()
	// Replace the annocation with the reference that we know we can use to
	// lookup the image in the cache.
	desc.Annotations[ocispec.AnnotationRefName] = reference
	return desc, nil
}

func (c *ImageCache) repository(ref registry.Reference) oras.ReadOnlyTarget {
	if c.RemoteRepository != nil {
		return c.RemoteRepository(ref)
	}
	return &remote.Repository{Reference: ref}
}

var (
	_ oras.ReadOnlyTarget = (*ImageCache)(nil)
)

// FetchImage retrieves an image from src. The image is identified by the given
// reference and filtered to match the specified platform.
//
// The function returns the pair of the image definition and the list of
// descriptors to retrieve the image layers (see ExtractLayers).
//
// The platform must not be nil or the function errors.
func FetchImage(ctx context.Context, src oras.ReadOnlyTarget, ref string, p *ocispec.Platform) (image *ocispec.Image, layers []ocispec.Descriptor, err error) {
	if p == nil {
		return nil, nil, fmt.Errorf("%s: must select a platform", ref)
	}

	options := oras.DefaultFetchOptions
	options.TargetPlatform = p

	_, rawManifest, err := oras.Fetch(ctx, src, ref, options)
	if err != nil {
		return nil, nil, err
	}
	defer rawManifest.Close()
	manifest := new(ocispec.Manifest)

	if err := json.NewDecoder(rawManifest).Decode(manifest); err != nil {
		return nil, nil, fmt.Errorf("%s: decoding image manifest: %w", ref, err)
	}

	rawConfig, err := src.Fetch(ctx, manifest.Config)
	if err != nil {
		return nil, nil, err
	}
	defer rawConfig.Close()

	image = new(ocispec.Image)
	switch manifest.Config.MediaType {
	case ocispec.MediaTypeImageConfig, "application/vnd.docker.container.image.v1+json":
		if err := json.NewDecoder(rawConfig).Decode(&image); err != nil {
			return nil, nil, err
		}
	default:
		return nil, nil, fmt.Errorf("%s: %s: %w", ref, manifest.Config.MediaType, errdef.ErrUnsupported)
	}
	return image, manifest.Layers, nil
}
