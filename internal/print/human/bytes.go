package human

import (
	"encoding"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"strconv"

	yaml "gopkg.in/yaml.v3"
)

// Bytes represents a number of bytes.
//
// The type support parsing values in formats like:
//
//	42 KB
//	8Gi
//	1.5KiB
//	...
//
// Two models are supported, using factors of 1000 and factors of 1024 via units
// like KB, MB, GB for the former, or Ki, Mi, MiB for the latter.
//
// In the current implementation, formatting is always done in factors of 1024,
// using units like Ki, Mi, Gi etc...
//
// Values may be decimals when using units larger than B. Partial bytes cannot
// be represnted (e.g. 0.5B is not supported).
type Bytes uint64

const (
	B Bytes = 1

	KB Bytes = 1000 * B
	MB Bytes = 1000 * KB
	GB Bytes = 1000 * MB
	TB Bytes = 1000 * GB
	PB Bytes = 1000 * TB

	KiB Bytes = 1024 * B
	MiB Bytes = 1024 * KiB
	GiB Bytes = 1024 * MiB
	TiB Bytes = 1024 * GiB
	PiB Bytes = 1024 * TiB
)

func ParseBytes(s string) (Bytes, error) {
	f, err := ParseBytesFloat64(s)
	if err != nil {
		return 0, err
	}
	if f < 0 {
		return 0, fmt.Errorf("invalid negative byte count: %q", s)
	}
	return Bytes(math.Floor(f)), err
}

func ParseBytesFloat64(s string) (float64, error) {
	value, unit := parseUnit(s)

	scale := Bytes(0)
	switch {
	case match(unit, "B"), unit == "":
		scale = B
	case match(unit, "KB"):
		scale = KB
	case match(unit, "MB"):
		scale = MB
	case match(unit, "GB"):
		scale = GB
	case match(unit, "TB"):
		scale = TB
	case match(unit, "PB"):
		scale = PB
	case match(unit, "KiB"):
		scale = KiB
	case match(unit, "MiB"):
		scale = MiB
	case match(unit, "GiB"):
		scale = GiB
	case match(unit, "TiB"):
		scale = TiB
	case match(unit, "PiB"):
		scale = PiB
	default:
		return 0, fmt.Errorf("malformed bytes representation: %q", s)
	}

	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed bytes representations: %q: %w", s, err)
	}
	return f * float64(scale), nil
}

type byteUnit struct {
	scale Bytes
	unit  string
}

var bytes1000 = [...]byteUnit{
	{B, "B"},
	{KB, "KB"},
	{MB, "MB"},
	{GB, "GB"},
	{TB, "TB"},
	{PB, "PB"},
}

var bytes1024 = [...]byteUnit{
	{B, "B"},
	{KiB, "KiB"},
	{MiB, "MiB"},
	{GiB, "GiB"},
	{TiB, "TiB"},
	{PiB, "PiB"},
}

func (b Bytes) String() string {
	return b.formatWith(bytes1024[:])
}

func (b Bytes) GoString() string {
	return fmt.Sprintf("human.Bytes(%d)", uint64(b))
}

// Format satisfies the fmt.Formatter interface.
//
// The method supports the following formatting verbs:
//
//	d	base 10, unit-less
//	b	base 10, with unit using 1000 factors
//	s	base 10, with unit using 1024 factors (same as calling String)
//	v	same as the 's' format, unless '#' is set to print the go value
func (b Bytes) Format(w fmt.State, v rune) {
	_, _ = io.WriteString(w, b.format(w, v))
}

func (b Bytes) format(w fmt.State, v rune) string {
	switch v {
	case 'd':
		return strconv.FormatUint(uint64(b), 10)
	case 'b':
		return b.formatWith(bytes1000[:])
	case 's':
		return b.formatWith(bytes1024[:])
	case 'v':
		if w.Flag('#') {
			return b.GoString()
		}
		return b.format(w, 's')
	default:
		return printError(v, b, uint64(b))
	}
}

func (b Bytes) formatWith(units []byteUnit) string {
	var scale Bytes
	var unit string

	for i := len(units) - 1; i >= 0; i-- {
		u := units[i]

		if b >= u.scale {
			scale, unit = u.scale, u.unit
			break
		}
	}

	s := ftoa(float64(b), float64(scale))
	if unit != "" {
		s += " " + unit
	}
	return s
}

func (b Bytes) Get() any {
	return uint64(b)
}

func (b *Bytes) Set(s string) error {
	p, err := ParseBytes(s)
	if err != nil {
		return err
	}
	*b = p
	return nil
}

func (b Bytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(uint64(b))
}

func (b *Bytes) UnmarshalJSON(j []byte) error {
	return json.Unmarshal(j, (*uint64)(b))
}

func (b Bytes) MarshalYAML() (any, error) {
	return uint64(b), nil
}

func (b *Bytes) UnmarshalYAML(y *yaml.Node) error {
	var s string
	if err := y.Decode(&s); err != nil {
		return err
	}
	p, err := ParseBytes(s)
	if err != nil {
		return err
	}
	*b = p
	return nil
}

func (b Bytes) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *Bytes) UnmarshalText(t []byte) error {
	return b.Set(string(t))
}

var (
	_ fmt.Formatter  = Bytes(0)
	_ fmt.GoStringer = Bytes(0)
	_ fmt.Stringer   = Bytes(0)

	_ json.Marshaler   = Bytes(0)
	_ json.Unmarshaler = (*Bytes)(nil)

	_ yaml.Marshaler   = Bytes(0)
	_ yaml.Unmarshaler = (*Bytes)(nil)

	_ encoding.TextMarshaler   = Bytes(0)
	_ encoding.TextUnmarshaler = (*Bytes)(nil)

	_ flag.Value = (*Bytes)(nil)
	_ flag.Value = (*Bytes)(nil)
)

type ByteArray []byte

// Format satisfies the fmt.Formatter interface.
//
// The method supports the following formatting verbs:
//
//	d base 10
//	x base 16
//
// Optional width can be given to limit the number of bytes shown.
func (b ByteArray) Format(w fmt.State, v rune) {
	cut, found := w.Width()
	if cut > len(b) || !found {
		cut = len(b)
	}

	s := "["

	start := b[:cut]
	more := len(b) > len(start)
	f := "%#02" + string(v)

	for i, x := range start {
		if i > 0 {
			s += " "
		}
		s += fmt.Sprintf(f, x)
	}
	if more {
		s += "..."
	}
	s += "]"

	_, _ = io.WriteString(w, s)
}
