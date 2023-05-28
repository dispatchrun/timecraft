package human

import (
	"encoding"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	yaml "gopkg.in/yaml.v3"
)

const (
	Nanosecond  Duration = 1
	Microsecond Duration = 1000 * Nanosecond
	Millisecond Duration = 1000 * Microsecond
	Second      Duration = 1000 * Millisecond
	Minute      Duration = 60 * Second
	Hour        Duration = 60 * Minute
	Day         Duration = 24 * Hour
	Week        Duration = 7 * Day
)

// Duration is based on time.Duration, but supports parsing and formatting
// more human-friendly representations.
//
// Here are examples of supported values:
//
//	5m30s
//	1d
//	4 weeks
//	1.5y
//	...
//
// The current implementation does not support decimal values, however,
// contributions are welcome to add this feature.
//
// Time being what it is, months and years are hard to represent because their
// durations vary in unpredictable ways. This is why the package only exposes
// constants up to a 1 week duration. For the sake of accuracy, years and months
// are always represented relative to a given date. Technically, leap seconds
// can cause any unit above the second to be variable, but in order to remain
// mentaly sane, we chose to ignore this detail in the implementation of this
// package.
type Duration time.Duration

func ParseDuration(s string) (Duration, error) {
	return ParseDurationUntil(s, time.Now())
}

func ParseDurationUntil(s string, now time.Time) (Duration, error) {
	var d Duration
	var input = s

	if s == "0" {
		return 0, nil
	}

	for len(s) != 0 {
		// parse the next number

		n, r, err := parseFloat(s)
		if err != nil {
			return 0, fmt.Errorf("malformed duration: %s: %w", input, err)
		}
		s = r

		// parse "weeks", "days", "h", etc.
		if s == "" {
			return 0, fmt.Errorf("please include a unit ('weeks', 'h', 'm') in addition to the value (%f)", n)
		}
		v, r, err := parseDuration(s, n, now)
		if err != nil {
			return 0, fmt.Errorf("malformed duration: %s: %w", input, err)
		}
		s = r

		d += v
	}

	return d, nil
}

func parseDuration(s string, n float64, now time.Time) (Duration, string, error) {
	s, r := parseNextToken(s)
	switch {
	case match(s, "weeks"):
		return Duration(n * float64(Week)), r, nil
	case match(s, "days"):
		return Duration(n * float64(Day)), r, nil
	case match(s, "hours"):
		return Duration(n * float64(Hour)), r, nil
	case match(s, "minutes"):
		return Duration(n * float64(Minute)), r, nil
	case match(s, "seconds"):
		return Duration(n * float64(Second)), r, nil
	case match(s, "milliseconds"), s == "ms":
		return Duration(n * float64(Millisecond)), r, nil
	case match(s, "microseconds"), s == "us", s == "µs":
		return Duration(n * float64(Microsecond)), r, nil
	case match(s, "nanoseconds"), s == "ns":
		return Duration(n * float64(Nanosecond)), r, nil
	case match(s, "months"):
		month, day := math.Modf(n)
		month, day = -month, -math.Round(28*day) // 1 month is approximately 4 weeks
		return Duration(now.Sub(now.AddDate(0, int(month), int(day)))), r, nil
	case match(s, "years"):
		year, month := math.Modf(n)
		year, month = -year, -math.Round(12*month)
		return Duration(now.Sub(now.AddDate(int(year), int(month), 0))), r, nil
	default:
		return 0, "", fmt.Errorf("unkonwn time unit %q", s)
	}
}

type durationUnits struct {
	nanosecond  string
	microsecond string
	millisecond string
	second      string
	minute      string
	hour        string
	day         string
	week        string
	month       string
	year        string
	separator   string
}

func (durationUnits) fix(n int, s string) string {
	if n == 1 && len(s) > 3 {
		return s[:len(s)-1] // trim tralinig 's' on long units
	}
	return s
}

var durationsShort = durationUnits{
	nanosecond:  "ns",
	microsecond: "µs",
	millisecond: "ms",
	second:      "s",
	minute:      "m",
	hour:        "h",
	day:         "d",
	week:        "w",
	month:       "mo",
	year:        "y",
	separator:   "",
}

var durationsLong = durationUnits{
	nanosecond:  "nanoseconds",
	microsecond: "microseconds",
	millisecond: "milliseconds",
	second:      "seconds",
	minute:      "minutes",
	hour:        "hours",
	day:         "days",
	week:        "weeks",
	month:       "months",
	year:        "years",
	separator:   " ",
}

func (d Duration) String() string {
	return d.text(time.Now(), 1, durationsShort)
}

func (d Duration) GoString() string {
	return fmt.Sprintf("human.Duration(%d)", int64(d))
}

// Format satisfies the fmt.Formatter interface.
//
// The method supports the following formatting verbs:
//
//	s	outputs a string representation of the duration (same as calling String)
//	v	same as the 's' format, unless '#' is set to print the go value
//
// The 's' and 'v' formatting verbs also interpret the options:
//
//	'-' outputs full names of the time units instead of abbreviations
//	'.' followed by a digit to limit the precision of the output
func (d Duration) Format(w fmt.State, v rune) {
	d.formatUntil(w, v, time.Now())
}

func (d Duration) formatUntil(w fmt.State, v rune, now time.Time) {
	_, _ = io.WriteString(w, d.format(w, v, now))
}

func (d Duration) format(w fmt.State, v rune, now time.Time) string {
	switch v {
	case 's':
		var limit int
		var units durationUnits

		limit, hasLimit := w.Precision()
		if !hasLimit {
			limit = 1
		}
		if w.Flag('+') {
			units = durationsLong
		} else {
			units = durationsShort
		}

		return d.text(now, limit, units)
	case 'v':
		if w.Flag('#') {
			return d.GoString()
		}
		return d.format(w, 's', now)
	default:
		return printError(v, d, uint64(d))
	}
}

func (d Duration) Text(now time.Time) string {
	return d.text(now, 1, durationsLong)
}

func (d Duration) text(now time.Time, limit int, units durationUnits) string {
	if d == 0 {
		return "0" + units.separator + units.second
	}

	if d == Duration(math.MaxInt64) || d == Duration(math.MinInt64) {
		return "a while" // special values for unknown durations
	}

	if d < 0 {
		return "-" + (-d).text(now, limit, units)
	}

	var n int
	var s strings.Builder

	for i := 0; d != 0; i++ {
		var unit string

		if i != 0 {
			s.WriteString(units.separator)
		}

		if d < 31*Day {
			var scale Duration

			switch {
			case d < Microsecond:
				scale, unit = Nanosecond, units.nanosecond
			case d < Millisecond:
				scale, unit = Microsecond, units.microsecond
			case d < Second:
				scale, unit = Millisecond, units.millisecond
			case d < Minute:
				scale, unit = Second, units.second
			case d < Hour:
				scale, unit = Minute, units.minute
			case d < Day:
				scale, unit = Hour, units.hour
			case d < Week:
				scale, unit = Day, units.day
			default:
				scale, unit = Week, units.week
			}

			n = int(d / scale)
			d -= Duration(n) * scale

		} else if n = d.Years(now); n != 0 {
			d -= Duration(now.Sub(now.AddDate(-n, 0, 0)))
			unit = units.year

		} else {
			n = d.Months(now)
			d -= Duration(now.Sub(now.AddDate(0, -n, 0)))
			unit = units.month
		}

		s.WriteString(strconv.Itoa(n))
		s.WriteString(units.separator)
		s.WriteString(units.fix(n, unit))

		if limit--; limit == 0 {
			break
		}
	}

	return s.String()
}

func (d Duration) Formatter(now time.Time) fmt.Formatter {
	return formatter(func(w fmt.State, v rune) { d.formatUntil(w, v, now) })
}

func (d *Duration) Set(s string) error {
	p, err := ParseDuration(s)
	if err != nil {
		return err
	}
	*d = p
	return nil
}

func (d Duration) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d))
}

func (d *Duration) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*time.Duration)(d))
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return time.Duration(d).String(), nil
}

func (d *Duration) UnmarshalYAML(y *yaml.Node) error {
	var s string
	if err := y.Decode(&s); err != nil {
		return err
	}
	p, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(p)
	return nil
}

func (d Duration) MarshalText() ([]byte, error) {
	return []byte(d.Text(time.Now())), nil
}

func (d *Duration) UnmarshalText(b []byte) error {
	return d.Set(string(b))
}

func (d Duration) Nanoseconds() int { return int(d) }

func (d Duration) Microseconds() int { return int(d) / int(Microsecond) }

func (d Duration) Milliseconds() int { return int(d) / int(Millisecond) }

func (d Duration) Seconds() int { return int(d) / int(Second) }

func (d Duration) Minutes() int { return int(d) / int(Minute) }

func (d Duration) Hours() int { return int(d) / int(Hour) }

func (d Duration) Days() int { return int(d) / int(Day) }

func (d Duration) Weeks() int { return int(d) / int(Week) }

func (d Duration) Months(until time.Time) int {
	if d < 0 {
		return -((-d).Months(until.Add(-time.Duration(d))))
	}

	cursor := until.Add(-time.Duration(d + 1))
	months := 0

	for cursor.Before(until) {
		cursor = cursor.AddDate(0, 1, 0)
		months++
	}

	return months - 1
}

func (d Duration) Years(until time.Time) int {
	if d < 0 {
		return -((-d).Years(until.Add(-time.Duration(d))))
	}

	cursor := until.Add(-time.Duration(d + 1))
	years := 0

	for cursor.Before(until) {
		cursor = cursor.AddDate(1, 0, 0)
		years++
	}

	return years - 1
}

var (
	_ fmt.Formatter  = Duration(0)
	_ fmt.GoStringer = Duration(0)
	_ fmt.Stringer   = Duration(0)

	_ json.Marshaler   = Duration(0)
	_ json.Unmarshaler = (*Duration)(nil)

	_ yaml.Marshaler   = Duration(0)
	_ yaml.Unmarshaler = (*Duration)(nil)

	_ encoding.TextMarshaler   = Duration(0)
	_ encoding.TextUnmarshaler = (*Duration)(nil)
	_ flag.Value               = (*Duration)(nil)
)
