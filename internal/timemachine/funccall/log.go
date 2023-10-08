package funccall

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// This number is arbitrary. For now we use the same segment as system calls ,
// any number larger than the largest system call will suffice.
const ID = 100

type Factory struct {
	enabled bool
	start   stack[time.Time]
	write   func(id int, ts time.Time, data []byte)
}

func (f *Factory) Enable(w func(id int, ts time.Time, data []byte)) {
	f.enabled = true
	f.write = w
}

var _ experimental.FunctionListenerFactory = (*Factory)(nil)

func (f *Factory) NewFunctionListener(fnd api.FunctionDefinition) experimental.FunctionListener {
	return &listener{
		f: f,
	}
}

type listener struct {
	f      *Factory
	buffer []byte
}

var _ experimental.FunctionListener = (*listener)(nil)

func (l *listener) Before(ctx context.Context,
	mod api.Module, def api.FunctionDefinition, params []uint64, _ experimental.StackIterator) {
	if !l.f.enabled {
		return
	}
	l.f.start.push(time.Now())
}

func (l *listener) After(ctx context.Context, mod api.Module, def api.FunctionDefinition, results []uint64) {
	if !l.f.enabled {
		return
	}
	start := l.f.start.pop().(time.Time)
	l.buffer = l.buffer[:0]

	// 4 bytes - depth ( for indentation)
	// 8 bytes - duration
	// followed by the function name
	l.buffer = binary.BigEndian.AppendUint32(l.buffer, uint32(l.f.start.size()))
	l.buffer = binary.BigEndian.AppendUint64(l.buffer, uint64(time.Since(start)))
	l.buffer = append(l.buffer, []byte(def.DebugName())...)
	l.f.write(ID, start, l.buffer)
}

func (l *listener) Abort(ctx context.Context, mod api.Module, def api.FunctionDefinition, _ error) {
	if !l.f.enabled {
		return
	}
	l.f.start.pop()
}

type stack[T any] struct {
	slice []any
}

func (s *stack[T]) push(v T) { s.slice = append(s.slice, v) }

func (s *stack[T]) pop() any {
	i := len(s.slice) - 1
	v := s.slice[i]
	s.slice = s.slice[:i]
	return v
}

func (s *stack[T]) size() int { return len(s.slice) }
