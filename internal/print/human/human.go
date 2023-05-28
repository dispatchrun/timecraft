// Package human provides types that support parsing and formatting
// human-friendly representations of values in various units.
//
// The package only exposes type names that are not that common to find in Go
// programs (in our experience). For that reason, it can be interesting to
// import the package as '.' (dot) to inject the symbols in the namespace of the
// importer, especially in the common case where it's being used in the main
// package of a program, for example:
//
//	import (
//		. "github.com/segmentio/cli/human"
//	)
//
// This can help improve code readability by importing constants in the package
// namespace, allowing constructs like:
//
//	type clientConfig{
//		DialTimeout Duration
//		BufferSize  Bytes
//		RateLimit   Rate
//	}
//	...
//	config := clientConfig{
//		DialTimeout: 10 * Second,
//		BufferSize:  64 * KiB,
//		RateLimit:   20 * PerSecond,
//	}
package human

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

func isDot(r rune) bool {
	return r == '.'
}

func isExp(r rune) bool {
	return r == 'e' || r == 'E'
}

func isSign(r rune) bool {
	return r == '-' || r == '+'
}

func isNumberPrefix(r rune) bool {
	return isSign(r) || unicode.IsDigit(r)
}

func hasPrefixFunc(s string, f func(rune) bool) bool {
	for _, r := range s {
		return f(r)
	}
	return false
}

func countPrefixFunc(s string, f func(rune) bool) int {
	var i int
	var r rune

	terminated := false
	for i, r = range s {
		if !f(r) {
			terminated = true
			break
		}
	}
	if !terminated {
		return i + 1
	}

	return i
}

func skipSpaces(s string) string {
	return strings.TrimLeftFunc(s, unicode.IsSpace)
}

func trimSpaces(s string) string {
	return strings.TrimRightFunc(s, unicode.IsSpace)
}

func parseNextNumber(s string) (string, string) {
	i := 0

	// integer part
	i += countPrefixFunc(s[i:], isSign) // - or +
	i += countPrefixFunc(s[i:], unicode.IsDigit)

	// Count all of the digits after the decimal (if one exists)
	if hasPrefixFunc(s[i:], isDot) {
		i++ // .
		i += countPrefixFunc(s[i:], unicode.IsDigit)
	}

	// exponent part
	if hasPrefixFunc(s[i:], isExp) {
		i++                                 // e or E
		i += countPrefixFunc(s[i:], isSign) // - or +
		i += countPrefixFunc(s[i:], unicode.IsDigit)
	}

	return s[:i], skipSpaces(s[i:])
}

func parseNextToken(s string) (string, string) {
	if hasPrefixFunc(s, isNumberPrefix) {
		return parseNextNumber(s)
	}

	for i, r := range s {
		if isNumberPrefix(r) || unicode.IsSpace(r) {
			return s[:i], skipSpaces(s[i:])
		}
	}

	return s, ""
}

// parseFloat tries to parse a number at the beginning of s, and returns the
// remainder as well as any error that occurs.
func parseFloat(s string) (float64, string, error) {
	s, r := parseNextNumber(s)
	f, err := strconv.ParseFloat(s, 64)
	return f, r, err
}

func parseUnit(s string) (head, unit string) {
	i := strings.LastIndexFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r)
	})

	if i < 0 {
		head = s
		return
	}

	head = trimSpaces(s[:i+1])
	unit = s[i+1:]
	return
}

func match(s, pattern string) bool {
	return len(s) <= len(pattern) && strings.EqualFold(s, pattern[:len(s)])
}

type suffix byte

func (c suffix) trim(s string) string {
	for len(s) > 0 && s[len(s)-1] == byte(c) {
		s = s[:len(s)-1]
	}
	return s
}

func (c suffix) match(s string) bool {
	return len(s) > 0 && s[len(s)-1] == byte(c)
}

func fabs(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}

func ftoa(value, scale float64) string {
	var format string

	if value == 0 {
		return "0"
	}

	if value < 0 {
		return "-" + ftoa(-value, scale)
	}

	switch {
	case (value / scale) >= 100:
		format = "%.0f"
	case (value / scale) >= 10:
		format = "%.1f"
	case scale > 1:
		format = "%.2f"
	default:
		format = "%.3f"
	}

	s := fmt.Sprintf(format, value/scale)
	if strings.Contains(s, ".") {
		s = suffix('0').trim(s)
		s = suffix('.').trim(s)
	}
	return s
}

func printError(verb rune, typ, val any) string {
	return fmt.Sprintf("%%!%c(%T=%v)", verb, typ, val)
}

type formatter func(fmt.State, rune)

func (f formatter) Format(w fmt.State, v rune) { f(w, v) }
