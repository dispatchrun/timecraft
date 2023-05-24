package wasicall

import (
	"context"

	. "github.com/stealthrocket/wasi-go"
)

// Observer wraps a wasi.System for observation.
//
// The provided callback receives a Syscall instance with details
// about a system call each time one is made. The callback is called after
// the system call returns but before control is returned to the guest.
// This allows the observer to see both system call inputs and outputs.
// It also allows for other action to be taken prior to system calls
// returning, for example pausing when provided an interactive debugging
// experience.
type Observer struct {
	System

	observe func(context.Context, Syscall)
}

func NewObserver(system System, observe func(context.Context, Syscall)) *Observer {
	return &Observer{system, observe}
}

var _ System = (*Observer)(nil)
var _ SocketsExtension = (*Observer)(nil)

func (o *Observer) ArgsSizesGet(ctx context.Context) (int, int, Errno) {
	argCount, stringBytes, errno := o.System.ArgsSizesGet(ctx)
	o.observe(ctx, &ArgsSizesGetSyscall{argCount, stringBytes, errno})
	return argCount, stringBytes, errno
}

func (o *Observer) ArgsGet(ctx context.Context) ([]string, Errno) {
	args, errno := o.System.ArgsGet(ctx)
	o.observe(ctx, &ArgsGetSyscall{args, errno})
	return args, errno
}

func (o *Observer) EnvironSizesGet(ctx context.Context) (int, int, Errno) {
	envCount, stringBytes, errno := o.System.EnvironSizesGet(ctx)
	o.observe(ctx, &EnvironSizesGetSyscall{envCount, stringBytes, errno})
	return envCount, stringBytes, errno
}

func (o *Observer) EnvironGet(ctx context.Context) ([]string, Errno) {
	env, errno := o.System.EnvironGet(ctx)
	o.observe(ctx, &EnvironGetSyscall{env, errno})
	return env, errno
}
