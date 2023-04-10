package timecraft

import (
	"context"
	"net/http"
	"sync"

	"github.com/stealthrocket/wazergo"
)

var hostModule wazergo.HostModule[*Module] = functions{
	"error_is_temporary": wazergo.F1((*Module).errorIsTemporary),
	"error_is_timeout":   wazergo.F1((*Module).errorIsTimeout),
	"error_message_size": wazergo.F1((*Module).errorMessageSize),
	"error_message_read": wazergo.F2((*Module).errorMessageRead),

	"promise_create":  wazergo.F0((*Module).promiseCreate),
	"promise_resolve": wazergo.F1((*Module).promiseResolve),
	"promise_discard": wazergo.F1((*Module).promiseDiscard),
	"promise_wait":    wazergo.F1((*Module).promiseWait),

	"http_client_create":                         wazergo.F12((*Module).httpClientCreate),
	"http_client_shutdown":                       wazergo.F1((*Module).httpClientShutdown),
	"http_client_exchange_create":                wazergo.F3((*Module).httpClientExchangeCreate),
	"http_client_excahnge_write_request_header":  wazergo.F5((*Module).httpClientExchangeWriteRequestHeader),
	"http_client_exchange_write_request_body":    wazergo.F3((*Module).httpClientExchangeWriteRequestBody),
	"http_client_exchange_write_request_trailer": wazergo.F2((*Module).httpClientExchangeWriteRequestTrailer),
	"http_client_exchange_send_request":          wazergo.F1((*Module).httpClientExchangeSendRequest),
	"http_client_exchange_read_response_header":  wazergo.F7((*Module).httpClientExchangeReadResponseHeader),
	"http_client_exchange_read_response_body":    wazergo.F3((*Module).httpClientExchangeReadResponseBody),
	"http_client_exchange_read_response_trailer": wazergo.F2((*Module).httpClientExchangeReadResponseTrailer),
	"http_client_exchange_close":                 wazergo.F1((*Module).httpClientExchangeClose),

	"http_server_create":                          wazergo.F7((*Module).httpServerCreate),
	"http_server_shutdown":                        wazergo.F1((*Module).httpServerShutdown),
	"http_server_exchange_create":                 wazergo.F2((*Module).httpServerExchangeCreate),
	"http_server_exchange_read_request_header":    wazergo.F5((*Module).httpServerExchangeReadRequestHeader),
	"http_server_exchange_read_request_body":      wazergo.F3((*Module).httpServerExchangeReadRequestBody),
	"http_server_exchange_read_request_trailer":   wazergo.F2((*Module).httpServerExchangeReadRequestTrailer),
	"http_server_exchange_write_response_header":  wazergo.F5((*Module).httpServerExchangeWriteResponseHeader),
	"http_server_exchange_write_response_body":    wazergo.F3((*Module).httpServerExchangeWriteResponseBody),
	"http_server_exchange_write_response_trailer": wazergo.F2((*Module).httpServerExchangeWriteResponseTrailer),
	"http_server_exchange_close":                  wazergo.F1((*Module).httpServerExchangeClose),
}

type functions wazergo.Functions[*Module]

func (f functions) Name() string {
	return "timecraft"
}

func (f functions) Functions() wazergo.Functions[*Module] {
	return (wazergo.Functions[*Module])(f)
}

func (f functions) Instantiate(ctx context.Context, opts ...Option) (*Module, error) {
	mod := &Module{
		Transport: http.DefaultTransport,
		promises:  make(map[int64]context.CancelCauseFunc),
		ready:     make(chan int64),
	}
	mod.ctx, mod.cancel = context.WithCancel(context.Background())
	wazergo.Configure(mod, opts...)
	return mod, nil
}

type Option = wazergo.Option[*Module]

type Module struct {
	group    sync.WaitGroup
	mutex    sync.Mutex
	lastID   int64
	promises map[int64]context.CancelCauseFunc
	ready    chan int64
	ctx      context.Context
	cancel   context.CancelFunc

	Transport http.RoundTripper
}

func (m *Module) Close(ctx context.Context) error {
	m.cancel()
	m.group.Wait()
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for id := range m.promises {
		delete(m.promises, id)
	}
	return nil
}

func (m *Module) spawn(f func(context.Context)) int64 {
	ctx, cancel := context.WithCancelCause(m.ctx)
	m.group.Add(1)
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.lastID++
	id := m.lastID
	m.promises[id] = cancel

	go func() {
		defer m.group.Done()

		f(ctx)
		cancel(nil)

		if ctx.Err() != discard {
			select {
			case <-m.ctx.Done():
			case m.ready <- id:
			}
		}
	}()
	return id
}
