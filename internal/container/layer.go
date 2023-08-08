package container

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/errdef"
)

// RootFS returns a slice of files opened on the tar files of layers making the
// root file system of a container image.
//
// The function automatically fetches and extracts missing layers using the src
// store and the list of descriptors representing the compressed image layers.
//
// The dst parameter is the directory where the layers are extracted, it must
// exist or the function will error.
func RootFS(ctx context.Context, dst string, src oras.ReadOnlyTarget, image *ocispec.Image, layers []ocispec.Descriptor) ([]*os.File, error) {
	files := make([]*os.File, len(image.RootFS.DiffIDs))
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()

	for {
		var missingLayers []ocispec.Descriptor

		for i, diffID := range image.RootFS.DiffIDs {
			if files[i] != nil {
				continue
			}
			// Attempt to open the layer file, if we succeed the layer already
			// existed in the cache directory and there is no need to fetch it
			// from the object store.
			f, err := os.Open(layerPath(dst, diffID))
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					missingLayers = append(missingLayers, layers[i])
					continue
				}
				return nil, err
			}
			files[i] = f
		}

		if len(missingLayers) == 0 {
			ret := files
			files = nil // prevents the defer from closing the files
			return ret, nil
		}

		// Some of the layer files were missing, we need to fetch and extract
		// them from the object store. On success, the returned list of open
		// files is guaranteed to have as many files as there were missing
		// layers, with a 1:1 correspondence on the indexes.
		missingFiles, err := ExtractLayers(ctx, dst, src, missingLayers)
		if err != nil {
			return nil, err
		}

		j := 0
		for i, f := range files {
			if f == nil {
				files[i] = missingFiles[j]
				j++
			}
		}
	}
}

// ExtractLayers fetches the image layers represented by the list of descriptors
// and extracts them at the dst file path.
//
// The content of the layers is written in a directory named after their digest
// algorithm, and uncompressed in a tar archive named with their digest hash.
//
//	(dst)
//	 └── sha256
//	    └── 8c6d1654570f041603f4cef49c320c8f6f3e401324913009d92a19132cbf1ac0
//	        ...
//
// The function performs the extraction of each layer concurrently, each layer
// is processed in a dedicated goroutine.
//
// The operation is atomic, either the creation of the layer fully succeeds or
// it fails and the layer is not present in the directory.
func ExtractLayers(ctx context.Context, dst string, src oras.ReadOnlyTarget, layers []ocispec.Descriptor) ([]*os.File, error) {
	files := make([]*os.File, len(layers))
	errch := make(chan error)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i, desc := range layers {
		go func(i int, desc ocispec.Descriptor) {
			f, err := ExtractLayer(ctx, dst, src, desc)
			if err != nil {
				cancel()
				err = fmt.Errorf("%s: extract: %w", desc.Digest, err)
			} else {
				files[i] = f
			}
			errch <- err
		}(i, desc)
	}

	var errs []error
	for range layers {
		if err := <-errch; err != nil {
			errs = append(errs, err)
		}
	}

	if err := errors.Join(errs...); err != nil {
		for _, f := range files {
			if f != nil {
				f.Close()
			}
		}
		return nil, err
	}

	return files, nil
}

// ExtractLayer is like ExtractLayers but applied to a single layer.
func ExtractLayer(ctx context.Context, dst string, src oras.ReadOnlyTarget, desc ocispec.Descriptor) (*os.File, error) {
	algo := desc.Digest.Algorithm()
	hash := desc.Digest.Encoded()
	path := filepath.Join(dst, algo.String())

	if err := os.Mkdir(path, 0777); err != nil {
		if !errors.Is(err, fs.ErrExist) {
			return nil, err
		}
	}

	f, err := os.CreateTemp(path, "."+hash+".*")
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// This boolean controls whether we need to remove the temporary file, it is
	// set to true after the archive was extracted and moved to its final path,
	// in which case the temporary file path does not exist anymore.
	success := false
	defer func() {
		if !success {
			os.Remove(f.Name())
		}
	}()

	r, err := FetchLayer(ctx, src, desc)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	digester := algo.Digester()

	if _, err := io.Copy(f, io.TeeReader(r, digester.Hash())); err != nil {
		return nil, err
	}

	digest := digester.Digest()
	path = layerPath(dst, digest)

	if err := os.Rename(f.Name(), path); err != nil {
		return nil, err
	}
	success = true
	return duplicateFile(f, path)
}

// FetchLayer returns a reader exposing the uncompressed content of an image
// layer retrieved from src and represented by desc.
func FetchLayer(ctx context.Context, src oras.ReadOnlyTarget, desc ocispec.Descriptor) (io.ReadCloser, error) {
	// https://github.com/opencontainers/image-spec/blob/main/layer.md#zstd-media-types
	const mediaTypeImageLayerZstd = "application/vnd.oci.image.layer.v1.tar+zstd"
	// Layers of docker images do not use the standard OCI media type
	const dockerMediaTypeLayer = "application/vnd.docker.image.rootfs.diff.tar.gzip"

	switch desc.MediaType {
	case ocispec.MediaTypeImageLayer:
	case ocispec.MediaTypeImageLayerGzip:
	case mediaTypeImageLayerZstd:
	case dockerMediaTypeLayer:
	default:
		return nil, fmt.Errorf("%s: %w", desc.MediaType, errdef.ErrUnsupported)
	}
	r, err := src.Fetch(ctx, desc)
	if err != nil {
		return nil, err
	}
	var z io.ReadCloser
	switch desc.MediaType {
	case ocispec.MediaTypeImageLayerGzip, dockerMediaTypeLayer:
		z, err = gzip.NewReader(r)
	case mediaTypeImageLayerZstd:
		z, err = zstdNewReader(r, zstd.WithDecoderConcurrency(1), zstd.WithDecoderLowmem(true))
	default:
		return r, nil
	}
	if err != nil {
		r.Close()
		return nil, fmt.Errorf("%s: %w", desc.Digest, err)
	}
	return &uncompressedLayer{uncompressed: z, compressed: r}, nil
}

type uncompressedLayer struct {
	uncompressed io.ReadCloser
	compressed   io.ReadCloser
}

func (r *uncompressedLayer) Read(b []byte) (int, error) {
	return r.uncompressed.Read(b)
}

func (r *uncompressedLayer) Close() error {
	r.uncompressed.Close()
	r.compressed.Close()
	return nil
}

func zstdNewReader(r io.Reader, opts ...zstd.DOption) (zstdReadCloser, error) {
	z, err := zstd.NewReader(r, opts...)
	return zstdReadCloser{z}, err
}

// This adapter is necessary to satisfy io.ReadCloser because zstd.Decoder.Close
// does not return an error and therfore does not implement io.Closer.
type zstdReadCloser struct{ *zstd.Decoder }

func (z zstdReadCloser) Close() error {
	z.Decoder.Close()
	return nil
}

func layerPath(dir string, id digest.Digest) string {
	return filepath.Join(dir, id.Algorithm().String(), id.Encoded())
}
