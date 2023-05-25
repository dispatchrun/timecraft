package assert

import (
	"errors"
	"reflect"
	"testing"

	"golang.org/x/exp/constraints"
)

func OK(t testing.TB, err error) {
	if err != nil {
		t.Helper()
		t.Fatal("error:", err)
	}
}

func Error(t testing.TB, got, want error) {
	if !errors.Is(got, want) {
		t.Helper()
		t.Fatalf("error mismatch\nwant = %s\ngot  = %s", want, got)
	}
}

func Equal[T comparable](t testing.TB, got, want T) {
	if got != want {
		t.Helper()
		t.Fatalf("value mismatch\nwant = %#v\ngot  = %#v", want, got)
	}
}

func EqualAll[T comparable](t testing.TB, got, want []T) {
	if len(got) != len(want) {
		t.Helper()
		t.Fatalf("number of values mismatch\nwant = %#v\ngot  = %#v", want, got)
	}

	for i, value := range want {
		if value != got[i] {
			t.Helper()
			t.Fatalf("value at index %d/%d mismatch\nwant = %#v\ngot  = %#v", i, len(want), value, got[i])
		}
	}
}

func Less[T constraints.Ordered](t testing.TB, less, more T) {
	if less >= more {
		t.Helper()
		t.Fatalf("value is too large: %v >= %v", less, more)
	}
}

func DeepEqual(t testing.TB, got, want any) {
	if !reflect.DeepEqual(got, want) {
		t.Helper()
		t.Fatalf("value mismatch\nwant = %#v\ngot  = %#v", want, got)
	}
}
