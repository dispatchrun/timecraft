package human

import (
	"encoding"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode"

	yaml "gopkg.in/yaml.v3"
)

// Time represents absolute point in times. The implementation is based on
// time.Time.
//
// The type supports all default time formats provided by the standard time
// package, as well as parsing and formatting values relative to a given time
// point, for example:
//
//	5 minutes ago
//	1h later
//	...
type Time time.Time

func ParseTime(s string) (Time, error) {
	return ParseTimeAt(s, time.Now())
}

func ParseTimeAt(s string, now time.Time) (Time, error) {
	if s == "now" {
		return Time(now), nil
	}

	if strings.HasSuffix(s, " ago") {
		s = strings.TrimLeftFunc(s[:len(s)-4], unicode.IsSpace)
		d, err := ParseDurationUntil(s, now)
		if err != nil {
			return Time{}, fmt.Errorf("malformed time representation: %q", s)
		}
		return Time(now.Add(-time.Duration(d))), nil
	}

	if strings.HasSuffix(s, " later") {
		s = strings.TrimRightFunc(s[:len(s)-6], unicode.IsSpace)
		d, err := ParseDurationUntil(s, now)
		if err != nil {
			return Time{}, fmt.Errorf("malformed time representation: %q", s)
		}
		return Time(now.Add(time.Duration(d))), nil
	}

	for _, format := range []string{
		time.ANSIC,
		time.UnixDate,
		time.RubyDate,
		time.RFC822,
		time.RFC822Z,
		time.RFC850,
		time.RFC1123,
		time.RFC1123Z,
		time.RFC3339,
		time.RFC3339Nano,
		time.Kitchen,
		time.Stamp,
		time.StampMilli,
		time.StampMicro,
		time.StampNano,
	} {
		t, err := time.Parse(format, s)
		if err == nil {
			return Time(t), nil
		}
	}

	return Time{}, fmt.Errorf("unsupported time representation: %q", s)
}

func (t Time) IsZero() bool {
	return time.Time(t).IsZero()
}

func (t Time) String() string {
	return t.text(time.Now(), Duration.String)
}

func (t Time) GoString() string {
	return fmt.Sprintf("human.Time{s:%d,ns:%d}",
		time.Time(t).Unix(),
		time.Time(t).Nanosecond())
}

// Format satisfies the fmt.Formatter interface.
//
// The method supports the following formatting verbs:
//
//	s	duration relative to now (same as calling String)
//	v	sam as the 's' format, unless '#' is set to print the go value
//
// The 's' and 'v' formatting verbs also interpret the options:
//
//	'-' outputs full names of the time units instead of abbreviations
//	'.' followed by a digit to limit the precision of the output
func (t Time) Format(w fmt.State, v rune) {
	t.formatAt(w, v, time.Now())
}

func (t Time) formatAt(w fmt.State, v rune, now time.Time) {
	_, _ = io.WriteString(w, t.format(w, v, now))
}

func (t Time) format(w fmt.State, v rune, now time.Time) string {
	switch v {
	case 's':
		return t.text(now, func(d Duration) string { return d.format(w, v, now) })
	case 'v':
		if w.Flag('#') {
			return t.GoString()
		}
		return t.format(w, 's', now)
	default:
		return printError(v, t, time.Time(t))
	}
}

func (t Time) Text(now time.Time) string {
	return t.text(now, func(d Duration) string { return d.Text(now) })
}

func (t Time) text(now time.Time, format func(Duration) string) string {
	if t.IsZero() {
		return "(none)"
	}
	d := Duration(now.Sub(time.Time(t)))
	switch {
	case d > 0:
		return format(d) + " ago"
	case d < 0:
		return format(-d) + " later"
	default:
		return "now"
	}
}

func (t Time) Formatter(now time.Time) fmt.Formatter {
	return formatter(func(w fmt.State, v rune) { t.formatAt(w, v, now) })
}

func (t *Time) Set(s string) error {
	p, err := ParseTime(s)
	if err != nil {
		return err
	}
	*t = p
	return nil
}

func (t Time) MarshalJSON() ([]byte, error) {
	return time.Time(t).MarshalJSON()
}

func (t *Time) UnmarshalJSON(b []byte) error {
	return ((*time.Time)(t)).UnmarshalJSON(b)
}

func (t Time) MarshalYAML() (interface{}, error) {
	return time.Time(t).Format(time.RFC3339Nano), nil
}

func (t *Time) UnmarshalYAML(y *yaml.Node) error {
	var s string
	if err := y.Decode(&s); err != nil {
		return err
	}
	p, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return err
	}
	*t = Time(p)
	return nil
}

func (t Time) MarshalText() ([]byte, error) {
	return []byte(t.Text(time.Now())), nil
}

func (t *Time) UnmarshalText(b []byte) error {
	return t.Set(string(b))
}

var (
	_ fmt.Formatter  = Time{}
	_ fmt.GoStringer = Time{}
	_ fmt.Stringer   = Time{}

	_ json.Marshaler   = Time{}
	_ json.Unmarshaler = (*Time)(nil)

	_ yaml.IsZeroer    = Time{}
	_ yaml.Marshaler   = Time{}
	_ yaml.Unmarshaler = (*Time)(nil)

	_ encoding.TextMarshaler   = Time{}
	_ encoding.TextUnmarshaler = (*Time)(nil)
	_ flag.Value               = (*Time)(nil)
)
