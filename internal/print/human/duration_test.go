package human

import (
	"encoding/json"
	"fmt"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

func TestDurationParse(t *testing.T) {
	for _, test := range []struct {
		in  string
		out Duration
	}{
		{in: "0", out: 0},

		{in: "1ns", out: Nanosecond},
		{in: "1µs", out: Microsecond},
		{in: "1ms", out: Millisecond},
		{in: "1s", out: Second},
		{in: "1m", out: Minute},
		{in: "1h", out: Hour},

		{in: "1d", out: 24 * Hour},
		{in: "2d", out: 48 * Hour},
		{in: "1w", out: 7 * 24 * Hour},
		{in: "2w", out: 14 * 24 * Hour},

		{in: "1 nanosecond", out: Nanosecond},
		{in: "1 microsecond", out: Microsecond},
		{in: "1 millisecond", out: Millisecond},
		{in: "1 second", out: Second},
		{in: "1 minute", out: Minute},
		{in: "1 hour", out: Hour},

		{in: "1 day", out: 24 * Hour},
		{in: "2 days", out: 48 * Hour},
		{in: "1 week", out: 7 * 24 * Hour},
		{in: "2 weeks", out: 14 * 24 * Hour},

		{in: "1m30s", out: 1*Minute + 30*Second},
		{in: "1.5m", out: 1*Minute + 30*Second},
	} {
		t.Run(test.in, func(t *testing.T) {
			d, err := ParseDuration(test.in)
			if err != nil {
				t.Fatal(err)
			}
			if d != test.out {
				t.Error("parsed duration mismatch:", d, "!=", test.out)
			}
		})
	}
}

func TestDurationError(t *testing.T) {
	_, err := ParseDuration("10")
	if err == nil {
		t.Fatal(err, "ParseDuration(10), expected error, got nil")
	}
	if want := "please include a unit ('weeks', 'h', 'm') in addition to the value (10.000000)"; err.Error() != want {
		t.Errorf(`ParseDuration("10"), got %q, want %q`, err.Error(), want)
	}
}

func TestDurationFormat(t *testing.T) {
	for _, test := range []struct {
		in  Duration
		fmt string
		out string
	}{
		{fmt: "%v", out: "0s", in: 0},

		{fmt: "%v", out: "1ns", in: Nanosecond},
		{fmt: "%v", out: "1µs", in: Microsecond},
		{fmt: "%v", out: "1ms", in: Millisecond},
		{fmt: "%v", out: "1s", in: Second},
		{fmt: "%v", out: "1m", in: Minute},
		{fmt: "%v", out: "1h", in: Hour},

		{fmt: "%v", out: "1d", in: 24 * Hour},
		{fmt: "%v", out: "2d", in: 48 * Hour},
		{fmt: "%v", out: "1w", in: 7 * 24 * Hour},
		{fmt: "%v", out: "2w", in: 14 * 24 * Hour},
		{fmt: "%v", out: "1mo", in: 33 * 24 * Hour},
		{fmt: "%v", out: "2mo", in: 66 * 24 * Hour},
		{fmt: "%v", out: "1y", in: 400 * 24 * Hour},
		{fmt: "%v", out: "2y", in: 800 * 24 * Hour},

		{fmt: "%v", out: "1m", in: 1*Minute + 30*Second},
		{fmt: "%+.1v", out: "2 hours", in: 2*Hour + 1*Minute + 30*Second},
		{fmt: "%+.2v", out: "2 hours 1 minute", in: 2*Hour + 1*Minute + 30*Second},
		{fmt: "%+.3v", out: "2 hours 1 minute 30 seconds", in: 2*Hour + 1*Minute + 30*Second},
		{fmt: "%#v", out: "human.Duration(60000000000)", in: 1 * Minute},
	} {
		t.Run(test.out, func(t *testing.T) {
			if s := fmt.Sprintf(test.fmt, test.in); s != test.out {
				t.Error("duration string mismatch:", s, "!=", test.out)
			}
		})
	}
}

func TestDurationJSON(t *testing.T) {
	testDurationEncoding(t, (2 * Hour), json.Marshal, json.Unmarshal)
}

func TestDurationYAML(t *testing.T) {
	testDurationEncoding(t, (2 * Hour), yaml.Marshal, yaml.Unmarshal)
}

func testDurationEncoding(t *testing.T, x Duration, marshal func(any) ([]byte, error), unmarshal func([]byte, any) error) {
	b, err := marshal(x)
	if err != nil {
		t.Fatal("marshal error:", err)
	}

	v := Duration(0)
	if err := unmarshal(b, &v); err != nil {
		t.Error("unmarshal error:", err)
	} else if v != x {
		t.Error("value mismatch:", v, "!=", x)
	}
}
