package object_test

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/stream"
	"golang.org/x/exp/slices"
)

func TestObjectStore(t *testing.T) {
	t.Run("dir", func(t *testing.T) {
		testObjectStore(t, func(t *testing.T) (object.Store, func()) {
			store, err := object.DirStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			return store, func() {}
		})
	})
}

func testObjectStore(t *testing.T, newStore func(*testing.T) (object.Store, func())) {
	tests := []struct {
		scenario string
		function func(*testing.T, context.Context, object.Store)
	}{
		{
			scenario: "an empty object store has no entries",
			function: testObjectStoreListEmpty,
		},

		{
			scenario: "deleting non existing objects from an empty store",
			function: testObjectStoreDeleteEmpty,
		},

		{
			scenario: "reading non existing objects returns ErrNotExist",
			function: testObjectStoreReadNotExist,
		},

		{
			scenario: "stating non existing objects returns ErrNotExist",
			function: testObjectStoreStatNotExist,
		},

		{
			scenario: "objects created can be listed",
			function: testObjectStoreCreateAndList,
		},

		{
			scenario: "objects created can be read",
			function: testObjectStoreCreateAndRead,
		},

		{
			scenario: "deleted objects cannot be listed anymore",
			function: testObjectStoreDeleteAndList,
		},

		{
			scenario: "objects being created are not visible when listing",
			function: testObjectStoreListWhileCreate,
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			store, teardown := newStore(t)
			defer teardown()

			ctx := context.Background()
			if deadline, ok := t.Deadline(); ok {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, deadline)
				defer cancel()
			}

			test.function(t, ctx, store)
		})
	}
}

func testObjectStoreListEmpty(t *testing.T, ctx context.Context, store object.Store) {
	items := readValues(t, store.ListObjects(ctx, "."))
	if len(items) != 0 {
		t.Errorf("too many objects: want=0 got=%d", len(items))
	}
}

func testObjectStoreDeleteEmpty(t *testing.T, ctx context.Context, store object.Store) {
	assert.OK(t, store.DeleteObject(ctx, "nope"))
	assert.OK(t, store.DeleteObject(ctx, "whatever"))
}

func testObjectStoreReadNotExist(t *testing.T, ctx context.Context, store object.Store) {
	_, err := store.ReadObject(ctx, "nope")
	assert.Error(t, err, object.ErrNotExist)
}

func testObjectStoreStatNotExist(t *testing.T, ctx context.Context, store object.Store) {
	_, err := store.StatObject(ctx, "nope")
	assert.Error(t, err, object.ErrNotExist)
}

func testObjectStoreCreateAndList(t *testing.T, ctx context.Context, store object.Store) {
	assert.OK(t, store.CreateObject(ctx, "test-1", strings.NewReader("")))
	assert.OK(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")))
	assert.OK(t, store.CreateObject(ctx, "test-3", strings.NewReader("BC")))

	objects := readValues(t, store.ListObjects(ctx, "."))
	clearCreatedAt(objects)
	sortObjectInfo(objects)

	assert.EqualAll(t, objects, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-2", Size: 1},
		{Name: "test-3", Size: 2},
	})
}

func testObjectStoreCreateAndRead(t *testing.T, ctx context.Context, store object.Store) {
	assert.OK(t, store.CreateObject(ctx, "test-1", strings.NewReader("")))
	assert.OK(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")))
	assert.OK(t, store.CreateObject(ctx, "test-3", strings.NewReader("BC")))

	test1, err := store.ReadObject(ctx, "test-1")
	assert.OK(t, err)
	assert.Equal(t, string(readBytes(t, test1)), "")

	test2, err := store.ReadObject(ctx, "test-2")
	assert.OK(t, err)
	assert.Equal(t, string(readBytes(t, test2)), "A")

	test3, err := store.ReadObject(ctx, "test-3")
	assert.OK(t, err)
	assert.Equal(t, string(readBytes(t, test3)), "BC")
}

func testObjectStoreDeleteAndList(t *testing.T, ctx context.Context, store object.Store) {
	assert.OK(t, store.CreateObject(ctx, "test-1", strings.NewReader("")))
	assert.OK(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")))
	assert.OK(t, store.CreateObject(ctx, "test-3", strings.NewReader("BC")))
	assert.OK(t, store.DeleteObject(ctx, "test-2"))

	objects := readValues(t, store.ListObjects(ctx, "."))
	clearCreatedAt(objects)
	sortObjectInfo(objects)

	assert.EqualAll(t, objects, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-3", Size: 2},
	})
}

func testObjectStoreListWhileCreate(t *testing.T, ctx context.Context, store object.Store) {
	assert.OK(t, store.CreateObject(ctx, "test-1", strings.NewReader("")))
	assert.OK(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")))

	r, w := io.Pipe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.OK(t, store.CreateObject(ctx, "test-3", r))
	}()

	io.WriteString(w, "H")

	beforeCreateObject := readValues(t, store.ListObjects(ctx, "."))
	clearCreatedAt(beforeCreateObject)
	sortObjectInfo(beforeCreateObject)

	assert.EqualAll(t, beforeCreateObject, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-2", Size: 1},
	})

	io.WriteString(w, "ello World!")
	w.Close()
	<-done

	afterCreateObject := readValues(t, store.ListObjects(ctx, "."))
	clearCreatedAt(afterCreateObject)
	sortObjectInfo(afterCreateObject)

	assert.EqualAll(t, afterCreateObject, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-2", Size: 1},
		{Name: "test-3", Size: 12},
	})
}

func readBytes(t *testing.T, r io.ReadCloser) []byte {
	t.Helper()
	defer r.Close()
	b, err := io.ReadAll(r)
	assert.OK(t, err)
	return b
}

func readValues[T any](t *testing.T, r stream.ReadCloser[T]) []T {
	t.Helper()
	defer r.Close()
	items, err := stream.ReadAll[T](r)
	assert.OK(t, err)
	return items
}

func clearCreatedAt(objects []object.Info) {
	for i := range objects {
		objects[i].CreatedAt = time.Time{}
	}
}

func sortObjectInfo(objects []object.Info) {
	slices.SortFunc(objects, func(a, b object.Info) bool {
		return a.Name < b.Name
	})
}
