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
	o.observe(ctx, &ArgsSizesGetSyscall{argCount, stringBytes, errno})
	return argCount, stringBytes, errno
}

func (o *Observer) ArgsGet(ctx context.Context) ([]string, Errno) {
	args, errno := o.system.ArgsGet(ctx)
	o.observe(ctx, &ArgsGetSyscall{args, errno})
	return args, errno
}

func (o *Observer) EnvironSizesGet(ctx context.Context) (int, int, Errno) {
	envCount, stringBytes, errno := o.system.EnvironSizesGet(ctx)
	o.observe(ctx, &EnvironSizesGetSyscall{envCount, stringBytes, errno})
	return envCount, stringBytes, errno
}

func (o *Observer) EnvironGet(ctx context.Context) ([]string, Errno) {
	env, errno := o.system.EnvironGet(ctx)
	o.observe(ctx, &EnvironGetSyscall{env, errno})
	return env, errno
}
