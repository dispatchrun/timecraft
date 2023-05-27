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
// generates FunctionCallBefore and FunctionCallAfter events for the provided
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
// FunctionCallBefore and FunctionCallAfter events for the provided
// Listener.
func NewFunctionListenerFactory(listener Listener) experimental.FunctionListenerFactory {
	return &functionListenerFactory{listener}
}

type functionListenerFactory struct {
	Listener
}

func (c *functionListenerFactory) NewListener(definition api.FunctionDefinition) experimental.FunctionListener {
	return &functionListener{Listener: c.Listener}
}

type functionListener struct {
	Listener

	// cached events
	before FunctionCallBefore
	after  FunctionCallAfter
}

func (l *functionListener) Before(ctx context.Context, mod api.Module, def api.FunctionDefinition, paramValues []uint64, stackIterator experimental.StackIterator) context.Context {
	l.before = FunctionCallBefore{mod, def, paramValues, stackIterator}
	l.Listener.OnEvent(ctx, &l.before)
	return ctx
}

func (l *functionListener) After(ctx context.Context, mod api.Module, def api.FunctionDefinition, err error, resultValues []uint64) {
	l.after = FunctionCallAfter{mod, def, resultValues}
	l.Listener.OnEvent(ctx, &l.after)
}

// WASIListener wraps a wasi.System to generate WASICallBefore and
// WASICallAfter events for the provided Listener.
func WASIListener(system wasi.System, listener Listener) wasi.System {
	var before WASICallBefore
	var after WASICallAfter
	return wasicall.NewObserver(system,
		func(ctx context.Context, syscall wasicall.Syscall) {
			before = WASICallBefore{syscall}
			listener.OnEvent(ctx, &before)
		},
		func(ctx context.Context, syscall wasicall.Syscall) {
			after = WASICallAfter{syscall}
			listener.OnEvent(ctx, &after)
		})
}
