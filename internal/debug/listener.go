package debug

import (
	"context"

	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// Listener is an Event listener.
//
// The Listener exists to bridge the gap between wazero's low-level
// experimental.FunctionListener and the higher-level wasi.System interface.
// The Listener can be connected to either or both sources of events and can
// be used to drive interactive debugging experiences. The design also allows
// for additional types of Event to be added in future, such as events
// triggered when doing instruction or line level stepping.
type Listener interface {
	// OnEvent is an Event handler.
	//
	// The Event and its fields must not be retained across calls.
	OnEvent(context.Context, Event)
}

// RegisterFunctionListener registers an experimental.FunctionListener that
// generates FunctionCallBeforeEvent and FunctionCallAfterEvent events for the provided
// Listener.
//
// The listener must be registered before the WebAssembly module is compiled.
func RegisterFunctionListener(ctx context.Context, listener Listener) context.Context {
	return context.WithValue(ctx,
		experimental.FunctionListenerFactoryKey{},
		NewFunctionListenerFactory(listener),
	)
}

// NewFunctionListenerFactory creates a experimental.FunctionListenerFactory
// that builds an experimental.FunctionListener that generates
// FunctionCallBeforeEvent and FunctionCallAfterEvent events for the provided
// Listener.
func NewFunctionListenerFactory(listener Listener) experimental.FunctionListenerFactory {
	return &functionListenerFactory{listener}
}

type functionListenerFactory struct {
	Listener
}

func (c *functionListenerFactory) NewFunctionListener(definition api.FunctionDefinition) experimental.FunctionListener {
	return &functionListener{Listener: c.Listener}
}

type functionListener struct {
	Listener

	// cached events
	before FunctionCallBeforeEvent
	after  FunctionCallAfterEvent
	abort  FunctionCallAbortEvent
}

func (l *functionListener) Before(ctx context.Context, mod api.Module, def api.FunctionDefinition, params []uint64, stackIterator experimental.StackIterator) {
	l.before = FunctionCallBeforeEvent{mod, def, params, stackIterator}
	l.Listener.OnEvent(ctx, &l.before)
}

func (l *functionListener) After(ctx context.Context, mod api.Module, def api.FunctionDefinition, results []uint64) {
	l.after = FunctionCallAfterEvent{mod, def, results}
	l.Listener.OnEvent(ctx, &l.after)
}

func (l *functionListener) Abort(ctx context.Context, mod api.Module, def api.FunctionDefinition, err error) {
	l.abort = FunctionCallAbortEvent{mod, def, err}
	l.Listener.OnEvent(ctx, &l.abort)
}

// WASIListener wraps a wasi.System to generate SystemCallBeforeEvent and
// SystemCallAfterEvent events for the provided Listener.
func WASIListener(system wasi.System, listener Listener) wasi.System {
	var before SystemCallBeforeEvent
	var after SystemCallAfterEvent
	return wasicall.NewObserver(system,
		func(ctx context.Context, syscall wasicall.Syscall) {
			before = SystemCallBeforeEvent{syscall}
			listener.OnEvent(ctx, &before)
		},
		func(ctx context.Context, syscall wasicall.Syscall) {
			after = SystemCallAfterEvent{syscall}
			listener.OnEvent(ctx, &after)
		})
}
