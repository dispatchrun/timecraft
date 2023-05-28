package human

import (
	"encoding/json"
	"fmt"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestRateParse(t *testing.T) {
	for _, test := range []struct {
		in  string
		out Rate
	}{
		{in: "0", out: 0},
		{in: "0/s", out: 0},
		{in: "1234/s", out: 1234},
		{in: "10.2K/s", out: 10200},
	} {
		t.Run(test.in, func(t *testing.T) {
			r, err := ParseRate(test.in)
			if err != nil {
				t.Fatal(err)
			}
			if r != test.out {
				t.Error("parsed rate mismatch:", r, "!=", test.out)
			}
		})
	}
}

func TestRateFormat(t *testing.T) {
	for _, test := range []struct {
		in   Rate
		fmt  string
		out  string
		unit Duration
	}{
		{in: 0, fmt: "%v", out: "0/s", unit: Second},
		{in: 1234, fmt: "%v", out: "1234/s", unit: Second},
		{in: 10234, fmt: "%v", out: "10.2K/s", unit: Second},
		{in: 0.1, fmt: "%v", out: "100/ms", unit: Millisecond},
		{in: 604800, fmt: "%v", out: "1/w", unit: Week},
		{in: 1512000, fmt: "%v", out: "2.5/w", unit: Week},
		{in: 25, fmt: "%s", out: "25/s", unit: Second},
		{in: 25, fmt: "%#v", out: "human.Rate(25)", unit: Second},
	} {
		t.Run(test.out, func(t *testing.T) {
			if s := fmt.Sprintf(test.fmt, test.in.Formatter(test.unit)); s != test.out {
				t.Error("formatted rate mismatch:", s, "!=", test.out)
			}
		})
	}
}

func TestRateJSON(t *testing.T) {
	testRateEncoding(t, Rate(1.234), json.Marshal, json.Unmarshal)
}

func TestRateYAML(t *testing.T) {
	testRateEncoding(t, Rate(1.234), yaml.Marshal, yaml.Unmarshal)
}

func testRateEncoding(t *testing.T, x Rate, marshal func(interface{}) ([]byte, error), unmarshal func([]byte, interface{}) error) {
	b, err := marshal(x)
	if err != nil {
		t.Fatal("marshal error:", err)
	}

	v := Rate(0)
	if err := unmarshal(b, &v); err != nil {
		t.Error("unmarshal error:", err)
	} else if v != x {
		t.Error("value mismatch:", v, "!=", x)
	}
}
