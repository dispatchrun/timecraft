package timecraft

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"unsafe"

	"github.com/stealthrocket/wazergo"
	. "github.com/stealthrocket/wazergo/types"
)

const httpModuleName = "timecraft_http"

func NewHttpModule() wazergo.HostModule[*HttpModule] {
	return functions{
		"do":     wazergo.F4((*HttpModule).Do),
		"size":   wazergo.F1((*HttpModule).Size),
		"status": wazergo.F1((*HttpModule).Status),
		"close":  wazergo.F1((*HttpModule).CloseHandle),
		"read":   wazergo.F3((*HttpModule).Read),
	}
}

type HttpModuleOption = wazergo.Option[*HttpModule]

type functions wazergo.Functions[*HttpModule]

func (f functions) Name() string {
	return httpModuleName
}

func (f functions) Functions() wazergo.Functions[*HttpModule] {
	return (wazergo.Functions[*HttpModule])(f)
}

func (f functions) Instantiate(ctx context.Context, opts ...HttpModuleOption) (*HttpModule, error) {
	mod := &HttpModule{
		responses: map[int32]response{},
	}
	wazergo.Configure(mod, opts...)
	return mod, nil
}

type response struct {
	status int32
	// format:
	//
	// <int32 size in bytes of following header>
	// header1\0value1\0
	// header2\0value2\0
	// ...
	// [body bytes...]
	content []byte
}

type HttpModule struct {
	nextHandle int32
	responses  map[int32]response
}

func (m *HttpModule) Close(ctx context.Context) error {
	return nil
}

func (m *HttpModule) Size(ctx context.Context, h Int32) Int32 {
	r, ok := m.responses[int32(h)]
	if !ok {
		return -1
	}
	return Int32(len(r.content))
}

func (m *HttpModule) Status(ctx context.Context, h Int32) Int32 {
	r, ok := m.responses[int32(h)]
	if !ok {
		return -1
	}
	return Int32(r.status)
}

func (m *HttpModule) CloseHandle(ctx context.Context, h Int32) Int32 {
	delete(m.responses, int32(h))
	return 0
}

func (m *HttpModule) Read(ctx context.Context, h Int32, p Pointer[Uint8], s Int32) Int32 {
	r, ok := m.responses[int32(h)]
	if !ok {
		return -1
	}
	if int(s) < len(r.content) {
		panic("guest need to allocate large enough buffer")
	}
	ok = p.Memory().Write(p.Offset(), r.content)
	if !ok {
		panic("memory write out of range")
	}
	return Int32(len(r.content))
}

func (m *HttpModule) do(ctx context.Context, method, url string, headers []byte, body []byte) (int32, response, error) {
	h := m.nextHandle
	m.nextHandle++

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewReader(body))
	if err != nil {
		return h, response{}, err
	}
	iterheaders(headers, req.Header.Add)

	r, err := http.DefaultClient.Do(req)
	if err != nil {
		return h, response{}, err
	}
	defer r.Body.Close()
	result := response{}
	result.status = int32(r.StatusCode)

	b := make([]byte, 4)
	c := 0
	for key, values := range r.Header {
		for _, value := range values {
			b = append(b, []byte(key)...)
			b = append(b, 0)
			b = append(b, []byte(value)...)
			b = append(b, 0)
			c++
		}
	}
	hdrsize := len(b) - 4
	binary.LittleEndian.PutUint32(b[:4], uint32(hdrsize))
	body, err = io.ReadAll(r.Body)

	result.content = append(b, body...)

	return h, result, err
}

func iterheaders(b []byte, f func(k, v string)) {
	for len(b) > 0 {
		i := bytes.IndexByte(b, 0)
		kb := b[:i]
		key := *(*string)(unsafe.Pointer(&kb))
		b = b[i+1:]
		i = bytes.IndexByte(b, 0)
		kv := b[:i]
		value := *(*string)(unsafe.Pointer(&kv))
		b = b[i+1:]
		f(key, value)
	}
}

func (m *HttpModule) Do(ctx context.Context, cmethod Pointer[Uint8], curl Pointer[Uint8], hdrbuf Bytes, bodybuf Bytes) Int32 {
	n := strlen(cmethod)
	method := string(cmethod.Slice(n))
	n = strlen(curl)
	url := string(curl.Slice(n))

	headers := ([]byte)(hdrbuf)
	body := ([]byte)(bodybuf)

	h, r, err := m.do(ctx, method, url, headers, body)
	if err != nil {
		r.status = -1
		r.content = []byte(err.Error())
	}
	m.responses[h] = r

	return Int32(h)
}

func strlen(p Pointer[Uint8]) int {
	for i := 0; ; i++ {
		if p.Index(i).Load() == 0 {
			return i
		}
	}
}
