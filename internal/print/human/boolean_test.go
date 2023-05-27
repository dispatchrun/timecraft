package human

import (
	"fmt"
	"testing"
)

func TestBooleanParse(t *testing.T) {
	for _, test := range []struct {
		in  string
		out Boolean
	}{
		{in: "true", out: true},
		{in: "True", out: true},
		{in: "TRUE", out: true},

		{in: "false", out: false},
		{in: "False", out: false},
		{in: "FALSE", out: false},

		{in: "yes", out: true},
		{in: "Yes", out: true},
		{in: "YES", out: true},

		{in: "no", out: false},
		{in: "No", out: false},
		{in: "NO", out: false},
	} {
		t.Run(test.in, func(t *testing.T) {
			b, err := ParseBoolean(test.in)
			if err != nil {
				t.Fatal(err)
			}
			if b != test.out {
				t.Error("parsed boolean mismatch:", b, "!=", test.out)
			}
		})
	}
}

func TestBooleanFormat(t *testing.T) {
	for _, test := range []struct {
		in  Boolean
		fmt string
		out string
	}{
		{in: true, fmt: "%s", out: "yes"},
		{in: true, fmt: "%t", out: "true"},

		{in: true, fmt: "%+s", out: "Yes"},
		{in: true, fmt: "%+t", out: "True"},

		{in: true, fmt: "%#s", out: "YES"},
		{in: true, fmt: "%#t", out: "TRUE"},

		{in: false, fmt: "%s", out: "no"},
		{in: false, fmt: "%t", out: "false"},

		{in: false, fmt: "%+s", out: "No"},
		{in: false, fmt: "%+t", out: "False"},

		{in: false, fmt: "%#s", out: "NO"},
		{in: false, fmt: "%#t", out: "FALSE"},
	} {
		t.Run(test.out, func(t *testing.T) {
			if s := fmt.Sprintf(test.fmt, test.in); s != test.out {
				t.Error("formatted boolean mismatch:", s, "!=", test.out)
			}
		})
	}
}
