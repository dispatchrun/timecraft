package human

import (
	"encoding/json"
	"fmt"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestRatioParse(t *testing.T) {
	for _, test := range []struct {
		in  string
		out Ratio
	}{
		{in: "0", out: 0},
		{in: "0%", out: 0},
		{in: "0.0%", out: 0},
		{in: "12.34%", out: 0.1234},
		{in: "100%", out: 1},
		{in: "200%", out: 2},
	} {
		t.Run(test.in, func(t *testing.T) {
			n, err := ParseRatio(test.in)
			if err != nil {
				t.Fatal(err)
			}
			if n != test.out {
				t.Error("parsed ratio mismatch:", n, "!=", test.out)
			}
		})
	}
}

func TestRatioFormat(t *testing.T) {
	for _, test := range []struct {
		in  Ratio
		fmt string
		out string
	}{
		{in: 0, fmt: "%v", out: "0%"},
		{in: 0.1234, fmt: "%v", out: "12.34%"},
		{in: 1, fmt: "%v", out: "100%"},
		{in: 2, fmt: "%v", out: "200%"},
		{in: 0.234, fmt: "%#v", out: "human.Ratio(0.234)"},
	} {
		t.Run(test.out, func(t *testing.T) {
			if s := fmt.Sprintf(test.fmt, test.in); s != test.out {
				t.Error("formatted ratio mismatch:", s, "!=", test.out)
			}
		})
	}
}

func TestRatioJSON(t *testing.T) {
	testRatioEncoding(t, Ratio(0.234), json.Marshal, json.Unmarshal)
}

func TestRatioYAML(t *testing.T) {
	testRatioEncoding(t, Ratio(0.234), yaml.Marshal, yaml.Unmarshal)
}

func testRatioEncoding(t *testing.T, x Ratio, marshal func(any) ([]byte, error), unmarshal func([]byte, any) error) {
	b, err := marshal(x)
	if err != nil {
		t.Fatal("marshal error:", err)
	}

	v := Ratio(0)
	if err := unmarshal(b, &v); err != nil {
		t.Error("unmarshal error:", err)
	} else if v != x {
		t.Error("value mismatch:", v, "!=", x)
	}
}
