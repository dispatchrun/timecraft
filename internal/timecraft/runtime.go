package timecraft

import (
	"context"
	"fmt"

	"github.com/tetratelabs/wazero"
)

// NewRuntime constructs a wazero.Runtime that's configured according
// to the provided timecraft Config.
func NewRuntime(ctx context.Context, config *Config) (wazero.Runtime, error) {
	runtimeConfig := wazero.NewRuntimeConfig()

	var cache wazero.CompilationCache
	if cachePath, ok := config.Cache.Location.Value(); ok {
		// The cache is an optimization, so if we encounter errors we notify the
		// user but still go ahead with the runtime instantiation.
		path, err := cachePath.Resolve()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve timecraft cache location: %w", err)
		} else {
			cache, err = createCacheDirectory(path)
			if err != nil {
				return nil, fmt.Errorf("failed to create timecraft cache directory: %w", err)
			} else {
				runtimeConfig = runtimeConfig.WithCompilationCache(cache)
			}
		}
	}

	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	if cache != nil {
		runtime = &runtimeWithCompilationCache{
			Runtime: runtime,
			cache:   cache,
		}
	}
	return runtime, nil
}

type runtimeWithCompilationCache struct {
	wazero.Runtime
	cache wazero.CompilationCache
}

func (r *runtimeWithCompilationCache) Close(ctx context.Context) error {
	if r.cache != nil {
		defer r.cache.Close(ctx)
	}
	return r.Runtime.Close(ctx)
}

func createCacheDirectory(path string) (wazero.CompilationCache, error) {
	if err := createDirectory(path); err != nil {
		return nil, err
	}
	return wazero.NewCompilationCacheWithDir(path)
}
