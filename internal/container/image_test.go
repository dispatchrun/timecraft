package container_test

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"testing"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/container"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/oci"
	"oras.land/oras-go/v2/errdef"
	"oras.land/oras-go/v2/registry"
)

var platform = &ocispec.Platform{
	Architecture: "arm64",
	OS:           "linux",
	Variant:      "v8",
}

func TestImageStore(t *testing.T) {
	ctx := context.Background()

	local, err := oci.New(t.TempDir())
	assert.OK(t, err)

	remote, err := oci.NewFromFS(ctx, os.DirFS("testdata/alpine:3.18.2"))
	assert.OK(t, err)

	options := oras.DefaultCopyOptions
	options.WithTargetPlatform(platform)

	srcTag := "3.18.2"
	dstTag := "registry.hub.docker.com/library/alpine:3.18.2"

	desc, err := oras.Copy(ctx, remote, srcTag, local, dstTag, options)
	assert.OK(t, err)

	testImageStore(t, local, desc, dstTag)
}

func TestImageCache(t *testing.T) {
	ctx := context.Background()

	local, err := oci.New(t.TempDir())
	assert.OK(t, err)

	remote, err := oci.NewFromFS(ctx, os.DirFS("testdata/alpine:3.18.2"))
	assert.OK(t, err)

	cache := &container.ImageCache{
		LocalStore:       local,
		TargetPlatform:   platform,
		RemoteRepository: func(registry.Reference) oras.ReadOnlyTarget { return remote },
	}

	tag := "registry.hub.docker.com/library/alpine:3.18.2"

	desc := ocispec.Descriptor{
		MediaType: "application/vnd.docker.distribution.manifest.v2+json",
		Digest:    "sha256:e3bd82196e98898cae9fe7fbfd6e2436530485974dc4fb3b7ddb69134eda2407",
		Size:      528,
		Annotations: map[string]string{
			"org.opencontainers.image.ref.name": tag,
		},
		Platform: platform,
	}

	// first run will populate the cache
	testImageStore(t, cache, desc, tag)

	// second run is expected to hit the cache and never access remote repositories
	cache.RemoteRepository = func(ref registry.Reference) oras.ReadOnlyTarget {
		panic(ref.String() + ": cache miss")
	}
	testImageStore(t, cache, desc, tag)
}

func testImageStore(t *testing.T, store oras.ReadOnlyTarget, desc ocispec.Descriptor, tag string) {
	ctx := context.Background()

	t.Run("the image tag can be resolved by the store", func(t *testing.T) {
		rslv, err := store.Resolve(ctx, tag)
		assert.OK(t, err)
		assert.DeepEqual(t, rslv, desc)
	})

	t.Run("an unknown image tag cannot be resolved by the store", func(t *testing.T) {
		_, err := store.Resolve(ctx, "unknown")
		assert.Error(t, err, errdef.ErrNotFound)
	})

	t.Run("the image descriptor exists in the store", func(t *testing.T) {
		exists, err := store.Exists(ctx, desc)
		assert.OK(t, err)
		assert.Equal(t, exists, true)
	})

	t.Run("another descriptor does not exist in the store", func(t *testing.T) {
		other := desc
		other.Digest = "sha256:5053b247d78b5e43b5543fec77c856ce70b8dc705d9f38336fa77736f25ff470"
		other.Annotations = nil
		otherExists, err := store.Exists(ctx, other)
		assert.OK(t, err)
		assert.Equal(t, otherExists, false)
	})

	t.Run("the image can be fetched from the store", func(t *testing.T) {
		image, layers, err := container.FetchImage(ctx, store, tag, platform)
		assert.OK(t, err)
		assert.Equal(t, len(layers), 1)
		assert.Equal(t, image.Platform.Architecture, "arm64")
		assert.Equal(t, image.Platform.OS, "linux")

		t.Run("the layers of the image can be fetched from the store", func(t *testing.T) {
			dst := t.TempDir()

			files, err := container.ExtractLayers(ctx, dst, store, layers)
			assert.OK(t, err)
			defer closeFiles(files)

			for _, f := range files {
				s, err := f.Stat()
				assert.OK(t, err)
				assert.True(t, s.Size() > 0)

				r := tar.NewReader(f)
				for {
					_, err := r.Next()
					if err != nil {
						assert.Error(t, err, io.EOF)
						break
					}
				}
			}
		})

		t.Run("the root file system of the image can be created and cached", func(t *testing.T) {
			dst := t.TempDir()

			rootFS1, err := container.RootFS(ctx, dst, store, image, layers)
			assert.OK(t, err)
			defer closeFiles(rootFS1)

			rootFS2, err := container.RootFS(ctx, dst, store, image, layers)
			assert.OK(t, err)
			defer closeFiles(rootFS2)

			assert.Equal(t, len(rootFS1), len(rootFS2))
			for i := range rootFS1 {
				assert.Equal(t, rootFS1[i].Name(), rootFS2[i].Name())
			}
		})
	})
}

func closeFiles(files []*os.File) {
	for _, f := range files {
		f.Close()
	}
}
