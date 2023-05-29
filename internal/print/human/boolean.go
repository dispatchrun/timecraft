package human

import (
	"encoding"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"strings"

	yaml "gopkg.in/yaml.v3"
)

// Boolean returns a boolean value.
//
// The type supports parsing values as "true", "false", "yes", or "no", all
// case insensitive.
type Boolean bool

func ParseBoolean(s string) (Boolean, error) {
	switch strings.ToLower(s) {
	case "true", "yes":
		return true, nil
	case "false", "no":
		return false, nil
	default:
		return false, fmt.Errorf("invalid boolean representation: %q", s)
	}
}

// String satisfies the fmt.Stringer interface, returns "yes" or "no".
func (b Boolean) String() string { return b.string("yes", "no") }

// GoString satisfies the fmt.GoStringer interface.
func (b Boolean) GoString() string {
	return fmt.Sprintf("human.Boolean(%t)", bool(b))
}

// Format satisfies the fmt.Formatter interface.
//
// The method supports the following formatting verbse:
//
//	s	"yes" or "no"
//	t	"true" or "false"
//	v	same as 's'
//
// For each of these options, these extra flags are also intepreted:
//
//	Capitalized     +
//	All uppercase   #
func (b Boolean) Format(w fmt.State, v rune) {
	_, _ = io.WriteString(w, b.format(w, v))
}

func (b Boolean) format(w fmt.State, v rune) string {
	switch v {
	case 's', 'v':
		switch {
		case w.Flag('#'):
			return b.string("YES", "NO")
		case w.Flag('+'):
			return b.string("Yes", "No")
		default:
			return b.string("yes", "no")
		}
	case 't':
		switch {
		case w.Flag('#'):
			return b.string("TRUE", "FALSE")
		case w.Flag('+'):
			return b.string("True", "False")
		default:
			return b.string("true", "false")
		}
	default:
		return printError(v, b, bool(b))
	}
}

func (b Boolean) string(t, f string) string {
	if b {
		return t
	}
	return f
}

func (b Boolean) Get() any {
	return bool(b)
}

func (b *Boolean) Set(s string) error {
	x, err := ParseBoolean(s)
	if err != nil {
		return err
	}
	*b = x
	return nil
}

func (b Boolean) MarshalJSON() ([]byte, error) {
	return []byte(b.string("true", "false")), nil
}

func (b *Boolean) UnmarshalJSON(j []byte) error {
	return json.Unmarshal(j, (*bool)(b))
}

func (b Boolean) MarshalYAML() (any, error) {
	return bool(b), nil
}

func (b *Boolean) UnmarshalYAML(y *yaml.Node) error {
	return y.Decode((*bool)(b))
}

func (b Boolean) MarshalText() ([]byte, error) {
	return []byte(b.String()), nil
}

func (b *Boolean) UnmarshalText(t []byte) error {
	return b.Set(string(t))
}

var (
	_ fmt.Formatter  = Boolean(false)
	_ fmt.GoStringer = Boolean(false)
	_ fmt.Stringer   = Boolean(false)

	_ json.Marshaler   = Boolean(false)
	_ json.Unmarshaler = (*Boolean)(nil)

	_ yaml.Marshaler   = Boolean(false)
	_ yaml.Unmarshaler = (*Boolean)(nil)

	_ encoding.TextMarshaler   = Boolean(false)
	_ encoding.TextUnmarshaler = (*Boolean)(nil)

	_ flag.Getter = (*Boolean)(nil)
	_ flag.Value  = (*Boolean)(nil)
)
