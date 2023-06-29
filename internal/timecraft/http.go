package timecraft

import "net/http"

// HTTPRequest is an HTTP request.
//
// It's intended to buffer a full request in memory in case it needs to
// be used multiple times, compared to a http.Request with a generic
// io.ReadCloser Body that can only be consumed once.
type HTTPRequest struct {
	Method  string
	Path    string // TODO: other parts of the URL, e.g. query
	Headers http.Header
	Body    []byte
	Port    int
}

func (*HTTPRequest) taskInput() {}

// HTTPResponse is an HTTP response.
type HTTPResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func (*HTTPResponse) taskOutput() {}
