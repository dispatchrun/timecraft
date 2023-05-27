package human

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	yaml "gopkg.in/yaml.v3"
)

func TestTimeParse(t *testing.T) {
	now := time.Now()
	end := now.Add(1 * time.Second)

	for _, test := range []struct {
		in  string
		out Duration
	}{
		{in: "now", out: 0},

		{in: "1ns ago", out: -Nanosecond},
		{in: "1µs ago", out: -Microsecond},
		{in: "1ms ago", out: -Millisecond},
		{in: "1s ago", out: -Second},
		{in: "1m ago", out: -Minute},
		{in: "1h ago", out: -Hour},

		{in: "1 nanosecond ago", out: -Nanosecond},
		{in: "1 microsecond ago", out: -Microsecond},
		{in: "1 millisecond ago", out: -Millisecond},
		{in: "1 second ago", out: -Second},
		{in: "1 minute ago", out: -Minute},
		{in: "1 hour ago", out: -Hour},

		{in: "1 day ago", out: -24 * Hour},
		{in: "2 days ago", out: -48 * Hour},
		{in: "1 week ago", out: -7 * 24 * Hour},
		{in: "2 weeks ago", out: -14 * 24 * Hour},

		{in: "0s later", out: 0},

		{in: "1ns later", out: Nanosecond},
		{in: "1µs later", out: Microsecond},
		{in: "1ms later", out: Millisecond},
		{in: "1s later", out: Second},
		{in: "1m later", out: Minute},
		{in: "1h later", out: Hour},

		{in: "1 nanosecond later", out: Nanosecond},
		{in: "1 microsecond later", out: Microsecond},
		{in: "1 millisecond later", out: Millisecond},
		{in: "1 second later", out: Second},
		{in: "1 minute later", out: Minute},
		{in: "1 hour later", out: Hour},

		{in: "1 day later", out: 24 * Hour},
		{in: "2 days later", out: 48 * Hour},
		{in: "1 week later", out: 7 * 24 * Hour},
		{in: "2 weeks later", out: 14 * 24 * Hour},

		{in: "1.5m ago", out: -1*Minute - 30*Second},

		{in: end.Format(time.RFC3339Nano), out: 1 * Second},
	} {
		t.Run(test.in, func(t *testing.T) {
			p, err := ParseTimeAt(test.in, now)
			if err != nil {
				t.Fatal(err)
			}
			if d := Duration(time.Time(p).Sub(now)); d != test.out {
				t.Error("parsed time delta mismatch:", d, "!=", test.out)
			}
		})
	}
}

func TestTimeFormat(t *testing.T) {
	now := time.Now()

	for _, test := range []struct {
		in  Duration
		fmt string
		out string
	}{
		{fmt: "%v", out: "now", in: 0},

		{fmt: "%v", out: "1ns ago", in: -Nanosecond},
		{fmt: "%v", out: "1µs ago", in: -Microsecond},
		{fmt: "%v", out: "1ms ago", in: -Millisecond},
		{fmt: "%v", out: "1s ago", in: -Second},
		{fmt: "%v", out: "1m ago", in: -Minute},
		{fmt: "%v", out: "1h ago", in: -Hour},

		{fmt: "%v", out: "1d ago", in: -24 * Hour},
		{fmt: "%v", out: "2d ago", in: -48 * Hour},
		{fmt: "%v", out: "1w ago", in: -7 * 24 * Hour},
		{fmt: "%v", out: "2w ago", in: -14 * 24 * Hour},
		{fmt: "%v", out: "1mo ago", in: -33 * 24 * Hour},
		{fmt: "%v", out: "2mo ago", in: -66 * 24 * Hour},
		{fmt: "%v", out: "1y ago", in: -400 * 24 * Hour},
		{fmt: "%v", out: "2y ago", in: -800 * 24 * Hour},

		{fmt: "%v", out: "1ns later", in: Nanosecond},
		{fmt: "%v", out: "1µs later", in: Microsecond},
		{fmt: "%v", out: "1ms later", in: Millisecond},
		{fmt: "%v", out: "1s later", in: Second},
		{fmt: "%v", out: "1m later", in: Minute},
		{fmt: "%v", out: "1h later", in: Hour},

		{fmt: "%v", out: "1d later", in: 24 * Hour},
		{fmt: "%v", out: "2d later", in: 48 * Hour},
		{fmt: "%v", out: "1w later", in: 7 * 24 * Hour},
		{fmt: "%v", out: "2w later", in: 14 * 24 * Hour},
		{fmt: "%v", out: "1mo later", in: 33 * 24 * Hour},
		{fmt: "%v", out: "2mo later", in: 66 * 24 * Hour},
		{fmt: "%v", out: "1y later", in: 400 * 24 * Hour},
		{fmt: "%v", out: "2y later", in: 800 * 24 * Hour},

		{fmt: "%v", out: "1m later", in: 1*Minute + 30*Second},
		{fmt: "%+.1v", out: "2 hours later", in: 2*Hour + 1*Minute + 30*Second},
		{fmt: "%+.2v", out: "2 hours 1 minute later", in: 2*Hour + 1*Minute + 30*Second},
		{fmt: "%+.3v", out: "2 hours 1 minute 30 seconds later", in: 2*Hour + 1*Minute + 30*Second},
	} {
		t.Run(test.out, func(t *testing.T) {
			if s := fmt.Sprintf(test.fmt, Time(now.Add(time.Duration(test.in))).Formatter(now)); s != test.out {
				t.Error("time string mismatch:", s, "!=", test.out)
			}
		})
	}
}

func TestTimeJSON(t *testing.T) {
	testTimeEncoding(t, Time(time.Now()), json.Marshal, json.Unmarshal)
}

func TestTimeYAML(t *testing.T) {
	testTimeEncoding(t, Time(time.Now()), yaml.Marshal, yaml.Unmarshal)
}

func testTimeEncoding(t *testing.T, x Time, marshal func(interface{}) ([]byte, error), unmarshal func([]byte, interface{}) error) {
	b, err := marshal(x)
	if err != nil {
		t.Fatal("marshal error:", err)
	}

	v := Time{}
	if err := unmarshal(b, &v); err != nil {
		t.Error("unmarshal error:", err)
	} else if !time.Time(v).Equal(time.Time(x)) {
		t.Error("value mismatch:", v, "!=", x)
	}
}
