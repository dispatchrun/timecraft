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

func TestStore(t *testing.T) {
	t.Run("CreateAndLookup", func(t *testing.T) {
		dir, err := object.DirStore(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		store := timemachine.NewStore(dir)

		testStoreCreateAndLookup(t, store,
			(*timemachine.Store).CreateModule,
			(*timemachine.Store).LookupModule,
			&format.Module{
				Code: []byte("1234567890"),
			})

		testStoreCreateAndLookup(t, store,
			(*timemachine.Store).CreateRuntime,
			(*timemachine.Store).LookupRuntime,
			&format.Runtime{
				Version: "timecraft v0.0.1",
			})

		testStoreCreateAndLookup(t, store,
			(*timemachine.Store).CreateConfig,
			(*timemachine.Store).LookupConfig,
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

		testStoreCreateAndLookup(t, store,
			(*timemachine.Store).CreateProcess,
			(*timemachine.Store).LookupProcess,
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

type createMethod[T any] func(*timemachine.Store, context.Context, T) (*format.Descriptor, error)

type lookupMethod[T any] func(*timemachine.Store, context.Context, format.Hash) (T, error)

func testStoreCreateAndLookup[T resource](t *testing.T, store *timemachine.Store, create createMethod[T], lookup lookupMethod[T], want T) {
	t.Run(reflect.TypeOf(want).Elem().String(), func(t *testing.T) {
		ctx := context.Background()

		d1, err := create(store, ctx, want)
		assert.OK(t, err)

		d2, err := store.LookupDescriptor(ctx, d1.Digest)
		assert.OK(t, err)
		assert.DeepEqual(t, d1, d2)

		got, err := lookup(store, ctx, d1.Digest)
		assert.OK(t, err)
		assert.DeepEqual(t, got, want)
	})
}
