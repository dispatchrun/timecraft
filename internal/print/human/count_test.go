package human

import (
	"encoding/json"
	"fmt"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestCountParse(t *testing.T) {
	for _, test := range []struct {
		in  string
		out Count
	}{
		{in: "0", out: 0},
		{in: "1234", out: 1234},
		{in: "10.2K", out: 10200},
	} {
		t.Run(test.in, func(t *testing.T) {
			c, err := ParseCount(test.in)
			if err != nil {
				t.Fatal(err)
			}
			if c != test.out {
				t.Error("parsed count mismatch:", c, "!=", test.out)
			}
		})
	}
}

func TestCountFormat(t *testing.T) {
	for _, test := range []struct {
		in  Count
		fmt string
		out string
	}{
		{in: 0, fmt: "%v", out: "0"},
		{in: 1234, fmt: "%v", out: "1234"},
		{in: 10234, fmt: "%v", out: "10.2K"},
		{in: 123456789, fmt: "%d", out: "123456789"},
		{in: 123456789, fmt: "%s", out: "123M"},
		{in: 123456789, fmt: "%#v", out: "human.Count(1.23456789e+08)"},
	} {
		t.Run(test.out, func(t *testing.T) {
			if s := fmt.Sprintf(test.fmt, test.in); s != test.out {
				t.Error("formatted count mismatch:", s, "!=", test.out)
			}
		})
	}
}

func TestCountJSON(t *testing.T) {
	testCountEncoding(t, Count(1), json.Marshal, json.Unmarshal)
}

func TestCountYAML(t *testing.T) {
	testCountEncoding(t, Count(1), yaml.Marshal, yaml.Unmarshal)
}

func testCountEncoding(t *testing.T, x Count, marshal func(any) ([]byte, error), unmarshal func([]byte, any) error) {
	b, err := marshal(x)
	if err != nil {
		t.Fatal("marshal error:", err)
	}

	v := Count(0)
	if err := unmarshal(b, &v); err != nil {
		t.Error("unmarshal error:", err)
	} else if v != x {
		t.Error("value mismatch:", v, "!=", x)
	}
}
