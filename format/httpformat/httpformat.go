package httpformat

import "github.com/stealthrocket/timecraft/format"

type Header map[string]string

type Request struct {
	Proto  string       `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Method string       `json:"method,omitempty"   yaml:"method,omitempty"`
	Path   string       `json:"path,omitempty"     yaml:"path,omitempty"`
	Header Header       `json:"header,omitempty"   yaml:"header,omitempty"`
	Body   format.Bytes `json:"body,omitempty"     yaml:"body,omitempty"`
}

type Response struct {
	Proto      string       `json:"protocol,omitempty"   yaml:"protocol,omitempty"`
	StatusCode int          `json:"statusCode,omitempty" yaml:"statusCode,omitempty"`
	StatusText string       `json:"statusText,omitempty" yaml:"statusText,omitempty"`
	Header     Header       `json:"header,omitempty"     yaml:"header,omitempty"`
	Body       format.Bytes `json:"body,omitempty"       yaml:"body,omitempty"`
}
