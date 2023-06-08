package tracing

type httpRequest struct {
	Proto  string            `json:"protocol,omitempty" yaml:"protocol,omitempty"`
	Method string            `json:"method,omitempty"   yaml:"method,omitempty"`
	Path   string            `json:"path,omitempty"     yaml:"path,omitempty"`
	Header map[string]string `json:"header,omitempty"   yaml:"header,omitempty"`
	Body   Bytes             `json:"body,omitempty"     yaml:"body,omitempty"`
}

type httpResponse struct {
	Proto      string            `json:"protocol,omitempty"   yaml:"protocol,omitempty"`
	StatusCode int               `json:"statusCode,omitempty" yaml:"statusCode,omitempty"`
	StatusText string            `json:"statusText,omitempty" yaml:"statusText,omitempty"`
	Header     map[string]string `json:"header,omitempty"     yaml:"header,omitempty"`
	Body       Bytes             `json:"body,omitempty"       yaml:"body,omitempty"`
}
