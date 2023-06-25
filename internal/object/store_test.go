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

		{
			scenario: "tagged objects are filtered when listing",
			function: testObjectStoreListTaggedObjects,
		},

		{
			scenario: "listing matches objects by key prefixes",
			function: testObjectStoreListByPrefix,
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

	objects := listObjects(t, ctx, store, ".")
	assert.DeepEqual(t, objects, []object.Info{
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

	objects := listObjects(t, ctx, store, ".")
	assert.DeepEqual(t, objects, []object.Info{
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

	_, err := io.WriteString(w, "H")
	assert.OK(t, err)

	beforeCreateObject := listObjects(t, ctx, store, ".")
	assert.DeepEqual(t, beforeCreateObject, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-2", Size: 1},
	})

	_, err = io.WriteString(w, "ello World!")
	assert.OK(t, err)
	assert.OK(t, w.Close())
	<-done

	afterCreateObject := listObjects(t, ctx, store, ".")
	assert.DeepEqual(t, afterCreateObject, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-2", Size: 1},
		{Name: "test-3", Size: 12},
	})
}

func testObjectStoreListTaggedObjects(t *testing.T, ctx context.Context, store object.Store) {
	assert.OK(t, store.CreateObject(ctx, "test-1", strings.NewReader(""))) // no tags
	assert.OK(t, store.CreateObject(ctx, "test-2", strings.NewReader("A"),
		object.Tag{"tag-1", "value-1"},
		object.Tag{"tag-2", "value-2"},
	))
	assert.OK(t, store.CreateObject(ctx, "test-3", strings.NewReader("BC"),
		object.Tag{"tag-1", "value-1"},
		object.Tag{"tag-1", "value-2"},
		object.Tag{"tag-2", "value-3"},
	))

	object1 := object.Info{Name: "test-1", Size: 0}
	object2 := object.Info{Name: "test-2", Size: 1, Tags: []object.Tag{{"tag-1", "value-1"}, {"tag-2", "value-2"}}}
	object3 := object.Info{Name: "test-3", Size: 2, Tags: []object.Tag{{"tag-1", "value-1"}, {"tag-1", "value-2"}, {"tag-2", "value-3"}}}

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "."),
		[]object.Info{object1, object2, object3})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, ".", object.MATCH("tag-1", "value-1")),
		[]object.Info{object2, object3})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, ".", object.MATCH("tag-1", "value-2")),
		[]object.Info{object3})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, ".", object.MATCH("tag-2", "value-2")),
		[]object.Info{object2})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, ".", object.MATCH("tag-2", "value-3")),
		[]object.Info{object3})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, ".",
			object.OR(
				object.MATCH("tag-2", "value-2"),
				object.MATCH("tag-2", "value-3"),
			),
		),
		[]object.Info{object2, object3})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, ".",
			object.AND(
				object.MATCH("tag-1", "value-2"),
				object.MATCH("tag-2", "value-3"),
			),
		),
		[]object.Info{object3})
}

func testObjectStoreListByPrefix(t *testing.T, ctx context.Context, store object.Store) {
	assert.OK(t, store.CreateObject(ctx, "test-1", strings.NewReader("")))
	assert.OK(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")))
	assert.OK(t, store.CreateObject(ctx, "test-3", strings.NewReader("BC")))
	assert.OK(t, store.CreateObject(ctx, "sub/key-1.0", strings.NewReader("hello")))
	assert.OK(t, store.CreateObject(ctx, "sub/key-1.1", strings.NewReader("world")))
	assert.OK(t, store.CreateObject(ctx, "sub/key-2.0", strings.NewReader("!")))

	object0 := object.Info{Name: "sub/", Size: 0}
	object1 := object.Info{Name: "test-1", Size: 0}
	object2 := object.Info{Name: "test-2", Size: 1}
	object3 := object.Info{Name: "test-3", Size: 2}
	object4 := object.Info{Name: "sub/key-1.0", Size: 5}
	object5 := object.Info{Name: "sub/key-1.1", Size: 5}
	object6 := object.Info{Name: "sub/key-2.0", Size: 1}

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "whatever"),
		[]object.Info{})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "."),
		[]object.Info{object0, object1, object2, object3})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "test"),
		[]object.Info{object1, object2, object3})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "test-"),
		[]object.Info{object1, object2, object3})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "test-1"),
		[]object.Info{object1})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "sub"),
		[]object.Info{object0})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "sub/"),
		[]object.Info{object4, object5, object6})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "sub/key-"),
		[]object.Info{object4, object5, object6})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "sub/key-1"),
		[]object.Info{object4, object5})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "sub/key-1.0"),
		[]object.Info{object4})

	assert.DeepEqual(t,
		listObjects(t, ctx, store, "sub/key-2"),
		[]object.Info{object6})
}

func listObjects(t *testing.T, ctx context.Context, store object.Store, prefix string, filters ...object.Filter) []object.Info {
	objects := readValues(t, store.ListObjects(ctx, prefix, filters...))
	clearCreatedAt(objects)
	sortObjectInfo(objects)
	return objects
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
