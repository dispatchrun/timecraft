package assert

import (
	"errors"
	"math"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"golang.org/x/exp/constraints"
)

func OK(t testing.TB, err error) {
	if err != nil {
		t.Helper()
		t.Fatal("error:", err)
	}
}

func True(t testing.TB, value bool) {
	t.Helper()
	Equal(t, value, true)
}

func False(t testing.TB, value bool) {
	t.Helper()
	Equal(t, value, false)
}

func Error(t testing.TB, got, want error) {
	if !errors.Is(got, want) {
		t.Helper()
		t.Fatalf("error mismatch\nwant = %v\ngot  = %v", want, got)
	}
}

func Equal[T comparable](t testing.TB, got, want T) {
	if got != want {
		t.Helper()
		t.Fatalf("%T value must be equal\nwant = %+v\ngot  = %+v", want, want, got)
	}
}

func NotEqual[T comparable](t testing.TB, got, want T) {
	if got == want {
		t.Helper()
		t.Fatalf("%T value must not be equal\nwant != %+v\ngot   = %+v", want, want, got)
	}
}

func FloatEqual[T ~float32 | ~float64](t testing.TB, got, want, epsilon T) {
	if math.Abs(float64(got-want)) > float64(epsilon) {
		t.Helper()
		t.Fatalf("%T value must be equal\nwant    = %+v\ngot     = %+v\nepsilon = %v\n", want, want, got, epsilon)
	}
}

func EqualAll[T comparable](t testing.TB, got, want []T) {
	var zero T

	if len(got) != len(want) {
		t.Helper()
		t.Fatalf("number of %T values mismatch\nwant = %+v\ngot  = %+v", zero, want, got)
	}

	for i, value := range want {
		if value != got[i] {
			t.Helper()
			t.Fatalf("%T value at index %d/%d mismatch\nwant = %+v\ngot  = %+v", zero, i, len(want), value, got[i])
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
		t.Fatalf("%T value must be deeply equal\nwant = %+v\ngot  = %+v", want, want, got)
	}
}

func ExitError(t testing.TB, got error, wantExitCode int) {
	switch e := got.(type) {
	case *exec.ExitError:
		if gotExitCode := e.ExitCode(); gotExitCode != wantExitCode {
			t.Helper()
			t.Fatalf("exit code mismatch\nwant = %d\ngot  = %d", wantExitCode, gotExitCode)
		}
	default:
		t.Helper()
		t.Fatalf("error mismatch\nwant = exec.ExitError{%d}\ngot  = %s", wantExitCode, got)
	}
}

func HasPrefix(t testing.TB, got, want string) {
	if !strings.HasPrefix(got, want) {
		t.Helper()
		t.Fatalf("prefix mismatch\nwant = %q\ngot  = %q", want, got)
	}
}
