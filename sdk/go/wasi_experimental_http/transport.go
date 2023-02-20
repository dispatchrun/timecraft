//go:build tinygo.wasm

package http

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"unsafe"
)

func init() {
	http.DefaultTransport = &Transport{}
}

type Transport struct{}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	status := int32(0)
	response := int32(-1)
	defer func() {
		if response >= 0 {
			__wasi_experimental_http_close(response)
		}
	}()

	var body buffer
	if req.Body != nil && req.Body != http.NoBody {
		bodyBuffer := new(bytes.Buffer)
		if req.ContentLength > 0 {
			bodyBuffer.Grow(int(req.ContentLength))
		}
		defer req.Body.Close()

		if _, err := bodyBuffer.ReadFrom(req.Body); err != nil {
			return nil, err
		}
		body = makeBuffer(bodyBuffer.Bytes())
	}

	headerBuffer := new(bytes.Buffer)
	req.Header.Write(headerBuffer)

	reqURL := req.URL.String()
	headers := makeBuffer(headerBuffer.Bytes())

	if err := __wasi_experimental_http_req(reqURL, req.Method, headers, body, &status, &response); err != __wasi_experimental_http_success {
		return nil, &url.Error{req.Method, reqURL, errors.New(errorToString(err))}
	}

	if headerBuffer.Cap() == 0 {
		headerBuffer.Grow(1024)
	}

	headerBufferLen := int32(0)
	for {
		headerBuffer.Reset()
		headerBufferCap := headerBuffer.Cap()
		headers = makeBuffer(headerBuffer.Bytes()[:headerBufferCap])
		err := __wasi_experimental_http_header_get_all(response, headers, &headerBufferLen)
		if err == __wasi_experimental_http_success {
			break
		}
		if err != __wasi_experimental_http_buffer_too_small {
			return nil, &url.Error{req.Method, reqURL, errors.New(errorToString(err))}
		}
		headerBuffer.Grow(2 * headerBufferCap)
	}

	res := &http.Response{
		ProtoMajor: 1,
		ProtoMinor: 1,
		Proto:      "HTTP/1.1",
		StatusCode: int(status),
		Status:     http.StatusText(int(status)),
		Header:     make(http.Header),
	}

	if err := parseHeader(res.Header, headers.bytes()[:headerBufferLen]); err != nil {
		return nil, &url.Error{req.Method, reqURL, err}
	}

	contentLength, _ := strconv.Atoi(res.Header.Get("Content-Length"))
	res.ContentLength = int64(contentLength)

	res.Body = &responseBody{res: response}
	response = -1
	return res, nil
}

type buffer struct {
	ptr *byte
	len int32
}

func makeBuffer(b []byte) (buf buffer) {
	if len(b) > 0 {
		buf.ptr = &b[0]
		buf.len = int32(len(b))
	}
	return buf
}

func (buf *buffer) bytes() []byte {
	return unsafe.Slice(buf.ptr, buf.len)
}

//go:wasm-module wasi_experimental_http
//go:export req
//go:noescape
func __wasi_experimental_http_req(url, method string, headers, body buffer, status *int32, res *int32) int32

//go:wasm-module wasi_experimental_http
//go:export close
//go:noescape
func __wasi_experimental_http_close(res int32) int32

//go:wasm-module wasi_experimental_http
//go:export header_get
//go:noescape
func __wasi_experimental_http_header_get(res int32, name string, value buffer, written *int32) int32

//go:wasm-module wasi_experimental_http
//go:export header_get_all
//go:noescape
func __wasi_experimental_http_header_get_all(res int32, header buffer, written *int32) int32

//go:wasm-module wasi_experimental_http
//go:export body_read
//go:noescape
func __wasi_experimental_http_body_read(res int32, data buffer, written *int32) int32

const (
	__wasi_experimental_http_success int32 = iota
	__wasi_experimental_http_invalid_handle
	__wasi_experimental_http_memory_not_found
	__wasi_experimental_http_memory_access_error
	__wasi_experimental_http_buffer_too_small
	__wasi_experimental_http_header_not_found
	__wasi_experimental_http_utf8_error
	__wasi_experimental_http_destination_not_allowed
	__wasi_experimental_http_invalid_method
	__wasi_experimental_http_invalid_encoding
	__wasi_experimental_http_invalid_url
	__wasi_experimental_http_request_error
	__wasi_experimental_http_runtime_error
	__wasi_experimental_http_too_many_sessions
)

func errorToString(err int32) string {
	switch err {
	case __wasi_experimental_http_success:
		return "success"
	case __wasi_experimental_http_invalid_handle:
		return "invalid handle"
	case __wasi_experimental_http_memory_not_found:
		return "memory not found"
	case __wasi_experimental_http_memory_access_error:
		return "memory access error"
	case __wasi_experimental_http_buffer_too_small:
		return "buffer too small"
	case __wasi_experimental_http_header_not_found:
		return "header not found"
	case __wasi_experimental_http_utf8_error:
		return "UTF-8 error"
	case __wasi_experimental_http_destination_not_allowed:
		return "destination not allowed"
	case __wasi_experimental_http_invalid_method:
		return "invalid method"
	case __wasi_experimental_http_invalid_encoding:
		return "invalid encoding"
	case __wasi_experimental_http_invalid_url:
		return "invalid URL"
	case __wasi_experimental_http_request_error:
		return "request error"
	case __wasi_experimental_http_runtime_error:
		return "runtime error"
	case __wasi_experimental_http_too_many_sessions:
		return "too many sessions"
	default:
		return "unknown"
	}
}

var (
	errMissingCRLF  = errors.New("missing CRLF sequence in http header")
	errMissingColon = errors.New("missing colon separator in http header")
)

func parseHeader(header http.Header, input []byte) error {
	CRLF := []byte{'\r', '\n'}

	for len(input) > 0 {
		i := bytes.Index(input, CRLF)
		if i == 0 {
			break // terminating \r\n sequence
		}
		if i < 0 { // malformed b
			return errMissingCRLF
		}
		h := input[:i]
		j := bytes.IndexByte(h, ':')
		if j < 0 {
			return errMissingColon
		}
		k := bytes.TrimSpace(h[:j])
		v := bytes.TrimSpace(h[j+1:])
		header.Add(http.CanonicalHeaderKey(string(k)), string(v))
		input = input[i+2:]
	}

	return nil
}

type responseBody struct {
	res int32
	wrn int32
}

func (r *responseBody) Read(b []byte) (int, error) {
	err := __wasi_experimental_http_body_read(r.res, makeBuffer(b), &r.wrn)
	written := int(r.wrn)
	switch {
	case err != __wasi_experimental_http_success:
		return written, errors.New(errorToString(err))
	case written == 0:
		return 0, io.EOF
	default:
		return written, nil
	}
}

func (r *responseBody) Close() error {
	__wasi_experimental_http_close(r.res)
	r.res = -1
	return nil
}
