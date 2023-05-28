package human

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"

	yaml "gopkg.in/yaml.v3"
)

// Count represents a count without a unit.
//
// The type supports parsing and formatting values like:
//
//	1234
//	10 K
//	1.5M
//	...
type Count float64

const (
	K Count = 1000
	M Count = 1000 * K
	G Count = 1000 * M
	T Count = 1000 * G
	P Count = 1000 * T
)

func ParseCount(s string) (Count, error) {
	value, unit := parseUnit(s)

	scale := Count(0)
	switch {
	case unit == "":
		scale = 1
	case match(unit, "K"):
		scale = K
	case match(unit, "M"):
		scale = M
	case match(unit, "G"):
		scale = G
	case match(unit, "T"):
		scale = T
	case match(unit, "P"):
		scale = P
	default:
		return 0, fmt.Errorf("malformed count representation: %q", s)
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed count representation: %q: %w", s, err)
	}
	return Count(f) * scale, nil
}

func (c Count) String() string {
	var scale Count
	var unit string
	var f = float64(c)

	switch c = Count(fabs(f)); {
	case c >= P:
		scale, unit = P, "P"
	case c >= T:
		scale, unit = T, "T"
	case c >= G:
		scale, unit = G, "G"
	case c >= M:
		scale, unit = M, "M"
	case c >= 10*K:
		scale, unit = K, "K"
	default:
		scale, unit = 1, ""
	}

	return ftoa(f, float64(scale)) + unit
}

func (c Count) GoString() string {
	return fmt.Sprintf("human.Count(%v)", float64(c))
}

// Format satisfies the fmt.Formatter interface.
//
// The method supports the following formatting verbs:
//
//	d	base 10, unit-less, rounded to the nearest integer
//	e	base 10, unit-less, scientific notation
//	f	base 10, unit-less, decimal notation
//	g	base 10, unit-less, act like 'e' or 'f' depending on scale
//	s	base 10, with unit (same as calling String)
//	v	same as the 's' format, unless '#' is set to print the go value
func (c Count) Format(w fmt.State, v rune) {
	_, _ = io.WriteString(w, c.format(w, v))
}

func (c Count) format(w fmt.State, v rune) string {
	switch v {
	case 'd':
		return ftoa(math.Round(float64(c)), 1)
	case 'e', 'f', 'g':
		return strconv.FormatFloat(float64(c), byte(v), -1, 64)
	case 's':
		return c.String()
	case 'v':
		if w.Flag('#') {
			return c.GoString()
		}
		return c.format(w, 's')
	default:
		return printError(v, c, float64(c))
	}
}

func (c Count) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(c))
}

func (c *Count) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*float64)(c))
}

func (c Count) MarshalYAML() (interface{}, error) {
	return c.String(), nil
}

func (c *Count) UnmarshalYAML(y *yaml.Node) error {
	var s string
	if err := y.Decode(&s); err != nil {
		return err
	}
	p, err := ParseCount(s)
	if err != nil {
		return err
	}
	*c = p
	return nil
}

func (c Count) MarshalText() ([]byte, error) {
	return []byte(c.String()), nil
}

func (c *Count) UnmarshalText(b []byte) error {
	p, err := ParseCount(string(b))
	if err != nil {
		return err
	}
	*c = p
	return nil
}

var (
	_ fmt.Formatter  = Count(0)
	_ fmt.GoStringer = Count(0)
	_ fmt.Stringer   = Count(0)

	_ json.Marshaler   = Count(0)
	_ json.Unmarshaler = (*Count)(nil)

	_ yaml.Marshaler   = Count(0)
	_ yaml.Unmarshaler = (*Count)(nil)

	_ encoding.TextMarshaler   = Count(0)
	_ encoding.TextUnmarshaler = (*Count)(nil)
)
