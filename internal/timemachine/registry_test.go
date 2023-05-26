package timemachine_test

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

func TestRegistry(t *testing.T) {
	t.Run("CreateAndLookup", func(t *testing.T) {
		dir, err := object.DirStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		reg := timemachine.NewRegistry(dir)

		testRegistryCreateAndLookup(t, reg,
			(*timemachine.Registry).CreateModule,
			(*timemachine.Registry).LookupModule,
			&format.Module{
				Code: []byte("1234567890"),
			})

		testRegistryCreateAndLookup(t, reg,
			(*timemachine.Registry).CreateRuntime,
			(*timemachine.Registry).LookupRuntime,
			&format.Runtime{
				Version: "timecraft v0.0.1",
			})

		testRegistryCreateAndLookup(t, reg,
			(*timemachine.Registry).CreateConfig,
			(*timemachine.Registry).LookupConfig,
			&format.Config{
				Runtime: &format.Descriptor{
					MediaType: format.TypeTimecraftRuntime,
					Digest:    format.SHA256([]byte(`{"version":"v0.0.1"}`)),
					Size:      20,
				},
				Modules: []*format.Descriptor{{
					MediaType: format.TypeTimecraftModule,
					Digest:    format.SHA256([]byte("1234567890")),
					Size:      10,
				}},
				Args: []string{
					"app.wasm",
				},
				Env: []string{
					"PATH=/bin:/usr/bin:/usr/local/bin",
					"PWD=/root",
				},
			})

		testRegistryCreateAndLookup(t, reg,
			(*timemachine.Registry).CreateProcess,
			(*timemachine.Registry).LookupProcess,
			&format.Process{
				ID:        uuid.New(),
				StartTime: time.Unix(1685053878, 0),
				Config: &format.Descriptor{
					MediaType: format.TypeTimecraftConfig,
					Digest:    format.SHA256([]byte("whatever")),
					Size:      42,
				},
			})
	})
}

type resource interface {
	format.ResourceMarshaler
	format.ResourceUnmarshaler
}

type createMethod[T any] func(*timemachine.Registry, context.Context, T) (*format.Descriptor, error)

type lookupMethod[T any] func(*timemachine.Registry, context.Context, format.Hash) (T, error)

func testRegistryCreateAndLookup[T resource](t *testing.T, reg *timemachine.Registry, create createMethod[T], lookup lookupMethod[T], want T) {
	t.Run(reflect.TypeOf(want).Elem().String(), func(t *testing.T) {
		ctx := context.Background()

		d1, err := create(reg, ctx, want)
		assert.OK(t, err)

		d2, err := reg.LookupDescriptor(ctx, d1.Digest)
		assert.OK(t, err)
		assert.DeepEqual(t, d1, d2)

		got, err := lookup(reg, ctx, d1.Digest)
		assert.OK(t, err)
		assert.DeepEqual(t, got, want)
	})
}
