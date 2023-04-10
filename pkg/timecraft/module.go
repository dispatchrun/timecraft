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
