package timecraft

import (
	"context"
	"io"
	"net/http"

	"github.com/stealthrocket/wazergo"
	. "github.com/stealthrocket/wazergo/types"
)

const httpModuleName = "timecraft_http"

func NewHttpModule() wazergo.HostModule[*HttpModule] {
	return functions{
		"do":     wazergo.F1((*HttpModule).Do),
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
	body   []byte
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
	return Int32(len(r.body))
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
	if int(s) < len(r.body) {
		panic("guest need to allocate large enough buffer")
	}
	ok = p.Memory().Write(p.Offset(), r.body)
	if !ok {
		panic("memory write out of range")
	}
	return Int32(len(r.body))
}

func (m *HttpModule) Do(ctx context.Context, p Pointer[Uint8]) Int32 {
	n := strlen(p)
	url := string(p.Slice(n))

	h := m.nextHandle
	m.nextHandle++

	result := response{}
	r, err := http.Get(url)
	if err == nil {
		defer r.Body.Close()
		result.body, err = io.ReadAll(r.Body)
		result.status = int32(r.StatusCode)
	}
	if err != nil {
		result.status = -1
		result.body = []byte(err.Error())
	}

	m.responses[h] = result
	return Int32(h)
}

func strlen(p Pointer[Uint8]) int {
	for i := 0; ; i++ {
		if p.Index(i).Load() == 0 {
			return i
		}
	}
}
