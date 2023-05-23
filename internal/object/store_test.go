package object_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stealthrocket/timecraft/internal/object"
	"golang.org/x/exp/slices"
)

func TestObjectStore(t *testing.T) {
	t.Run("dir", func(t *testing.T) {
		testObjectStore(t, func(t *testing.T) (object.Store, func()) {
			store, err := object.DirStore(t.TempDir(), object.ReadDirBufferSize(1))
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
			scenario: "deleting non existing objects from an emtpy store",
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
	items := assertItems(t, store.ListObjects(ctx, "."))
	if len(items) != 0 {
		t.Errorf("too many objects: want=0 got=%d", len(items))
	}
}

func testObjectStoreDeleteEmpty(t *testing.T, ctx context.Context, store object.Store) {
	assertError(t, store.DeleteObject(ctx, "nope"), nil)
	assertError(t, store.DeleteObject(ctx, "whatever"), nil)
}

func testObjectStoreReadNotExist(t *testing.T, ctx context.Context, store object.Store) {
	_, err := store.ReadObject(ctx, "nope")
	assertError(t, err, object.ErrNotExist)
}

func testObjectStoreStatNotExist(t *testing.T, ctx context.Context, store object.Store) {
	_, err := store.StatObject(ctx, "nope")
	assertError(t, err, object.ErrNotExist)
}

func testObjectStoreCreateAndList(t *testing.T, ctx context.Context, store object.Store) {
	assertError(t, store.CreateObject(ctx, "test-1", strings.NewReader("")), nil)
	assertError(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")), nil)
	assertError(t, store.CreateObject(ctx, "test-3", strings.NewReader("BC")), nil)

	objects := assertItems(t, store.ListObjects(ctx, "."))
	clearCreatedAt(objects)
	sortObjectInfo(objects)

	assertEqualAll(t, objects, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-2", Size: 1},
		{Name: "test-3", Size: 2},
	})
}

func testObjectStoreCreateAndRead(t *testing.T, ctx context.Context, store object.Store) {
	assertError(t, store.CreateObject(ctx, "test-1", strings.NewReader("")), nil)
	assertError(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")), nil)
	assertError(t, store.CreateObject(ctx, "test-3", strings.NewReader("BC")), nil)

	test1, err := store.ReadObject(ctx, "test-1")
	assertError(t, err, nil)
	assertEqual(t, string(assertReadAll(t, test1)), "")

	test2, err := store.ReadObject(ctx, "test-2")
	assertError(t, err, nil)
	assertEqual(t, string(assertReadAll(t, test2)), "A")

	test3, err := store.ReadObject(ctx, "test-3")
	assertError(t, err, nil)
	assertEqual(t, string(assertReadAll(t, test3)), "BC")
}

func testObjectStoreDeleteAndList(t *testing.T, ctx context.Context, store object.Store) {
	assertError(t, store.CreateObject(ctx, "test-1", strings.NewReader("")), nil)
	assertError(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")), nil)
	assertError(t, store.CreateObject(ctx, "test-3", strings.NewReader("BC")), nil)
	assertError(t, store.DeleteObject(ctx, "test-2"), nil)

	objects := assertItems(t, store.ListObjects(ctx, "."))
	clearCreatedAt(objects)
	sortObjectInfo(objects)

	assertEqualAll(t, objects, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-3", Size: 2},
	})
}

func testObjectStoreListWhileCreate(t *testing.T, ctx context.Context, store object.Store) {
	assertError(t, store.CreateObject(ctx, "test-1", strings.NewReader("")), nil)
	assertError(t, store.CreateObject(ctx, "test-2", strings.NewReader("A")), nil)

	r, w := io.Pipe()

	done := make(chan struct{})
	go func() {
		defer close(done)
		assertError(t, store.CreateObject(ctx, "test-3", r), nil)
	}()

	io.WriteString(w, "H")

	beforeCreateObject := assertItems(t, store.ListObjects(ctx, "."))
	clearCreatedAt(beforeCreateObject)
	sortObjectInfo(beforeCreateObject)

	assertEqualAll(t, beforeCreateObject, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-2", Size: 1},
	})

	io.WriteString(w, "ello World!")
	w.Close()
	<-done

	afterCreateObject := assertItems(t, store.ListObjects(ctx, "."))
	clearCreatedAt(afterCreateObject)
	sortObjectInfo(afterCreateObject)

	assertEqualAll(t, afterCreateObject, []object.Info{
		{Name: "test-1", Size: 0},
		{Name: "test-2", Size: 1},
		{Name: "test-3", Size: 12},
	})
}

func assertItems[T any](t *testing.T, iter object.Iter[T]) []T {
	t.Helper()
	items, err := object.Items(iter)
	assertError(t, err, nil)
	return items
}

func assertReadAll(t *testing.T, r io.ReadCloser) []byte {
	defer r.Close()
	b, err := io.ReadAll(r)
	assertError(t, err, nil)
	return b
}

func assertEqual[T comparable](t *testing.T, got, want T) {
	t.Helper()
	if got != want {
		t.Fatalf("items mismatch: want=%+v got=%+v", want, got)
	}
}

func assertEqualAll[T comparable](t *testing.T, got, want []T) {
	if len(got) != len(want) {
		t.Fatalf("number of items mismatch: want=%d got=%d", len(want), len(got))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("items at index %d mismatch: want=%+v got=%+v", i, want[i], got[i])
		}
	}
}

func assertError(t *testing.T, got, want error) {
	t.Helper()
	if !errors.Is(got, want) {
		t.Fatal(got)
	}
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
