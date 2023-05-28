package human

import (
	"encoding"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Number is similar to Count, but supports values with separators for
// readability purposes.
//
// The type supports parsing and formatting values likes:
//
//	123
//	1.5
//	2,000,000
//	...
type Number float64

func ParseNumber(s string) (Number, error) {
	r := strings.ReplaceAll(s, ",", "")
	f, err := strconv.ParseFloat(r, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed number: %s: %w", s, err)
	}
	return Number(f), nil
}

func (n Number) String() string {
	if n == 0 {
		return "0"
	}

	if n < 0 {
		return "-" + (-n).String()
	}

	if n <= 1e-3 || n >= 1e12 {
		return strconv.FormatFloat(float64(n), 'g', -1, 64)
	}

	i, d := math.Modf(float64(n))
	parts := make([]string, 0, 4)

	for u := uint64(i); u > 0; u /= 1000 {
		parts = append(parts, strconv.FormatUint(u%1000, 10))
	}

	for i, j := 0, len(parts)-1; i < j; {
		parts[i], parts[j] = parts[j], parts[i]
		i++
		j--
	}

	r := strings.Join(parts, ",")

	if d != 0 {
		r += "."
		r += suffix('0').trim(strconv.FormatUint(uint64(math.Round(d*1000)), 10))
	}

	return r
}

func (n Number) GoString() string {
	return fmt.Sprintf("human.Number(%v)", float64(n))
}

// Format satisfies the fmt.Formatter interface.
//
// The method supports the following formatting verbs:
//
//	e	base 10, separator-free, scientific notation
//	f	base 10, separator-free, decimal notation
//	g	base 10, separator-free, act like 'e' or 'f' depending on scale
//	s	base 10, with separators (same as calling String)
//	v	same as the 's' format, unless '#' is set to print the go value
func (n Number) Format(w fmt.State, v rune) {
	_, _ = io.WriteString(w, n.format(w, v))
}

func (n Number) format(w fmt.State, v rune) string {
	switch v {
	case 'e', 'f', 'g':
		return strconv.FormatFloat(float64(n), byte(v), -1, 64)
	case 's':
		return n.String()
	case 'v':
		if w.Flag('#') {
			return n.GoString()
		}
		return n.format(w, 's')
	default:
		return printError(v, n, float64(n))
	}
}

func (n Number) MarshalJSON() ([]byte, error) {
	return json.Marshal(float64(n))
}

func (n *Number) UnmarshalJSON(b []byte) error {
	return json.Unmarshal(b, (*float64)(n))
}

func (n Number) MarshalYAML() (interface{}, error) {
	return float64(n), nil
}

func (n *Number) UnmarshalYAML(y *yaml.Node) error {
	return y.Decode((*float64)(n))
}

func (n Number) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

func (n *Number) UnmarshalText(b []byte) error {
	p, err := ParseNumber(string(b))
	if err != nil {
		return err
	}
	*n = p
	return nil
}

var (
	_ fmt.Formatter  = Number(0)
	_ fmt.GoStringer = Number(0)
	_ fmt.Stringer   = Number(0)

	_ json.Marshaler   = Number(0)
	_ json.Unmarshaler = (*Number)(nil)

	_ yaml.Marshaler   = Number(0)
	_ yaml.Unmarshaler = (*Number)(nil)

	_ encoding.TextMarshaler   = Number(0)
	_ encoding.TextUnmarshaler = (*Number)(nil)
)
