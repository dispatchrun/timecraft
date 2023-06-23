package tracing

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

func TestHttp1ParseCRLF(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		ok       bool
		remain   string
	}{
		{
			scenario: "empty input",
			input:    "",
			ok:       false,
			remain:   "",
		},

		{
			scenario: "one CRLF sequence",
			input:    "\r\n",
			ok:       true,
			remain:   "",
		},

		{
			scenario: "two CRLF sequence",
			input:    "\r\n\r\n",
			ok:       true,
			remain:   "\r\n",
		},

		{
			scenario: "not starting with CRLF",
			input:    " \r\n",
			ok:       false,
			remain:   " \r\n",
		},

		{
			scenario: "CRLF followed by other characters",
			input:    "\r\n1234",
			ok:       true,
			remain:   "1234",
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			ok, remain := http1ParseCRLF([]byte(test.input))
			assert.Equal(t, ok, test.ok)
			assert.Equal(t, string(remain), test.remain)
		})
	}
}

func TestHttp1ParseLWS(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		ok       bool
		remain   string
	}{
		{
			scenario: "empty input",
			input:    "",
			ok:       false,
			remain:   "",
		},

		{
			scenario: "CRLF sequence not followed by a SP",
			input:    "\r\n",
			ok:       false,
			remain:   "\r\n",
		},

		{
			scenario: "CRLF sequence followed by a SP",
			input:    "\r\n ",
			ok:       true,
			remain:   "",
		},

		{
			scenario: "CRLF sequence followed by a HT",
			input:    "\r\n\t",
			ok:       true,
			remain:   "",
		},

		{
			scenario: "CRLF sequence followed by a SP and other characters",
			input:    "\r\n 1234",
			ok:       true,
			remain:   "1234",
		},

		{
			scenario: "CRLF sequence followed by a HT and other characters",
			input:    "\r\n\t1234",
			ok:       true,
			remain:   "1234",
		},

		{
			scenario: "not starting with a CRLF sequence",
			input:    "1234\r\n",
			ok:       false,
			remain:   "1234\r\n",
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			ok, remain := http1ParseLWS([]byte(test.input))
			assert.Equal(t, ok, test.ok)
			assert.Equal(t, string(remain), test.remain)
		})
	}
}

func TestHttp1ParseFieldName(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		name     string
		remain   string
	}{
		{
			scenario: "empty input",
			input:    "",
			name:     "",
			remain:   "",
		},

		{
			scenario: "input without a :",
			input:    "Content-Length",
			name:     "",
			remain:   "Content-Length",
		},

		{
			scenario: "input ending with a :",
			input:    "Content-Length:",
			name:     "Content-Length",
			remain:   "",
		},

		{
			scenario: "input with a field name and value",
			input:    "Content-Length: 42",
			name:     "Content-Length",
			remain:   " 42",
		},

		{
			scenario: "input with a field name and value finishing with one CRLF sequence",
			input:    "Content-Length: 42\r\n",
			name:     "Content-Length",
			remain:   " 42\r\n",
		},

		{
			scenario: "input with a field name and value finishing with two CRLF sequence",
			input:    "Content-Length: 42\r\n\r\n",
			name:     "Content-Length",
			remain:   " 42\r\n\r\n",
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			name, remain := http1ParseFieldName([]byte(test.input))
			assert.Equal(t, string(name), test.name)
			assert.Equal(t, string(remain), test.remain)
		})
	}
}

func TestHttp1ParseFieldValue(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		value    string
		remain   string
	}{
		{
			scenario: "empty input",
			input:    "",
			value:    "",
			remain:   "",
		},

		{
			scenario: "input with no spaces",
			input:    "something",
			value:    "something",
			remain:   "",
		},

		{
			scenario: "input with spaces",
			input:    "hello world",
			value:    "hello world",
			remain:   "",
		},

		{
			scenario: "input ending with one CRLF sequence",
			input:    "hello world\r\n",
			value:    "hello world",
			remain:   "\r\n",
		},

		{
			scenario: "input ending with two CRLF sequence",
			input:    "hello world\r\n\r\n",
			value:    "hello world",
			remain:   "\r\n\r\n",
		},

		{
			scenario: "input folded with a LWS sequence made of CRLF and SP",
			input:    "first line\r\n second line\r\n",
			value:    "first line second line",
			remain:   "\r\n",
		},

		{
			scenario: "input folded with a LWS sequence made of CRLF and HT",
			input:    "first line\r\n\tsecond line\r\n",
			value:    "first line second line",
			remain:   "\r\n",
		},

		{
			scenario: "input made of only SP characters",
			input:    "  \r\n",
			value:    "",
			remain:   "\r\n",
		},

		{
			scenario: "input made of only HT characters",
			input:    "\t\t\r\n",
			value:    "",
			remain:   "\r\n",
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			value, remain := http1ParseFieldValue([]byte(test.input))
			assert.Equal(t, string(value), test.value)
			assert.Equal(t, string(remain), test.remain)
		})
	}
}

func TestHttp1SplitStartLine(t *testing.T) {
	tests := []struct {
		scenario string
		input    string
		part1    string
		part2    string
		part3    string
	}{
		{
			scenario: "empty input",
			input:    "",
			part1:    "",
			part2:    "",
			part3:    "",
		},

		{
			scenario: "request line with only a method",
			input:    "GET",
			part1:    "GET",
			part2:    "",
			part3:    "",
		},

		{
			scenario: "request line with a method and path",
			input:    "GET /",
			part1:    "GET",
			part2:    "/",
			part3:    "",
		},

		{
			scenario: "full request line",
			input:    "GET /hello/world HTTP/1.1\r\n",
			part1:    "GET",
			part2:    "/hello/world",
			part3:    "HTTP/1.1",
		},

		{
			scenario: "full status line",
			input:    "HTTP/1.1 200 OK\r\n",
			part1:    "HTTP/1.1",
			part2:    "200",
			part3:    "OK",
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			part1, part2, part3 := http1SplitStartLine([]byte(test.input))
			assert.Equal(t, string(part1), test.part1)
			assert.Equal(t, string(part2), test.part2)
			assert.Equal(t, string(part3), test.part3)
		})
	}
}

func TestHttp1HeaderRange(t *testing.T) {
	type field struct {
		name, value string
	}

	tests := []struct {
		scenario string
		input    string
		fields   []field
	}{
		{
			scenario: "empty input",
			input:    "",
			fields:   nil,
		},

		{
			scenario: "valid message header",
			input: "Date: Thu, 08 Jun 2023 02:06:32 GMT\r\n" +
				"Content-Length: 14\r\n" +
				"Content-Type: text/plain; charset=utf-8\r\n" +
				"\r\n",
			fields: []field{
				{name: "Date", value: "Thu, 08 Jun 2023 02:06:32 GMT"},
				{name: "Content-Length", value: "14"},
				{name: "Content-Type", value: "text/plain; charset=utf-8"},
			},
		},

		{
			scenario: "header with empty value",
			input: "Accept-Encoding:\r\n" +
				"\r\n",
			fields: []field{
				{name: "Accept-Encoding", value: ""},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.scenario, func(t *testing.T) {
			var fields []field
			http1HeaderRange([]byte(test.input), func(name, value []byte) bool {
				fields = append(fields, field{
					name:  string(name),
					value: string(value),
				})
				return true
			})
			assert.EqualAll(t, fields, test.fields)
		})
	}
}
