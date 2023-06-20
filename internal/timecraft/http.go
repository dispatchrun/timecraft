package timecraft

import "net/http"

type HTTPRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}
