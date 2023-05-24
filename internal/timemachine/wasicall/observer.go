package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
)

// Observer wraps a wasi.System
type Observer struct {
	system System

	observe func(context.Context, Syscall)
}

var _ System = (*Observer)(nil)
var _ SocketsExtension = (*Observer)(nil)

func (o *Observer) ArgsSizesGet(ctx context.Context) (int, int, Errno) {
	argCount, stringBytes, errno := o.system.ArgsSizesGet(ctx)

	// TODO: construct Syscall
	// TODO: call observe()

	panic("not implemented")
}
