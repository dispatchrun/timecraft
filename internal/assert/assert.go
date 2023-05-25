package assert

import (
	"errors"
	"testing"
)

func OK(t *testing.T, err error) {
	if err != nil {
		t.Helper()
		t.Fatal("error:", err)
	}
}

func Error(t *testing.T, got, want error) {
	if !errors.Is(got, want) {
		t.Helper()
		t.Fatalf("error mismatch:\n\twant = %s\ngot  = %s", want, got)
	}
}

func Equal[T comparable](t *testing.T, got, want T) {
	if got != want {
		t.Helper()
		t.Fatalf("value mismatch:\n\twant = %#v\ngot  = %#v", want, got)
	}
}

func EqualAll[T comparable](t *testing.T, got, want []T) {
	if len(got) != len(want) {
		t.Helper()
		t.Fatalf("number of values mismatch:\t\nwant = %#v\ngot  = %#v", want, got)
	}

	for i, value := range want {
		if value != got[i] {
			t.Helper()
			t.Fatalf("value at index %d/%d mismatch:\n\twant = %#v\ngot  = %#v", i, len(want), value, got[i])
		}
	}
}
