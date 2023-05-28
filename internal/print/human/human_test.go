package human

import (
	"testing"
)

func TestParseNextToken(t *testing.T) {
	for _, test := range []struct {
		in   string
		head string
		tail string
	}{
		{in: "", head: "", tail: ""},
		{in: "a", head: "a", tail: ""},
		{in: "a b c", head: "a", tail: "b c"},
		{in: "abc123", head: "abc", tail: "123"},
		{in: "abc-123", head: "abc", tail: "-123"},
		{in: "abc+123", head: "abc", tail: "+123"},
		{in: "abc 123", head: "abc", tail: "123"},
		{in: "123abc", head: "123", tail: "abc"},
		{in: "+123abc", head: "+123", tail: "abc"},
		{in: "-123abc", head: "-123", tail: "abc"},
		{in: "123 abc", head: "123", tail: "abc"},
		{in: "123.abc", head: "123.", tail: "abc"},
		{in: "123.456abc", head: "123.456", tail: "abc"},
		{in: "123e4abc", head: "123e4", tail: "abc"},
		{in: "123E4abc", head: "123E4", tail: "abc"},
		{in: "-123.4e+56abc", head: "-123.4e+56", tail: "abc"},
	} {
		t.Run("", func(t *testing.T) {
			head, tail := parseNextToken(test.in)
			if head != test.head {
				t.Errorf("head mismatch: %q != %q", head, test.head)
			}
			if tail != test.tail {
				t.Errorf("tail mismatch: %q != %q", tail, test.tail)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	in := "10"
	n, _, err := parseFloat(in)
	if err != nil {
		t.Fatalf("parseFloat(%q): got %q, want nil", in, err)
	}
	if want := float64(10); n != want {
		t.Fatalf("parseFloat(%q): got %f, want %f", in, n, want)
	}
}
