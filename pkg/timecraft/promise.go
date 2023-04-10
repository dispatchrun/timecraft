//go:build !wasm

package timecraft

import (
	"context"
	"errors"

	. "github.com/stealthrocket/wazergo/types"
)

var (
	discard = errors.New("discard")
)

func (m *Module) promiseCreate(ctx context.Context) Int64 {
	return Int64(m.spawn(func(ctx context.Context) {
		<-ctx.Done() // wait for completion
	}))
}

func (m *Module) promiseResolve(ctx context.Context, id Int64) Int32 {
	m.mutex.Lock()
	cancel, ok := m.promises[int64(id)]
	m.mutex.Unlock()
	if !ok {
		return -1
	}
	cancel(nil)
	return 0
}

func (m *Module) promiseDiscard(ctx context.Context, id Int64) Int32 {
	m.mutex.Lock()
	cancel, ok := m.promises[int64(id)]
	delete(m.promises, int64(id))
	m.mutex.Unlock()
	if !ok {
		return -1
	}
	cancel(discard)
	return 0
}

func (m *Module) promiseWait(ctx context.Context, out Array[int64]) Int32 {
	if len(out) == 0 {
		return 0
	}

	n := 0
	defer func() {
		m.mutex.Lock()
		for _, id := range out[:n] {
			delete(m.promises, id)
		}
		m.mutex.Unlock()
	}()

	select {
	case id := <-m.ready:
		out[0] = id
		n++
	case <-ctx.Done():
		return -1
	}

	for n < len(out) {
		select {
		case id := <-m.ready:
			out[n] = id
			n++
		default:
			return Int32(n)
		}
	}

	return Int32(n)
}
