//go:build !wasm

package timecraft

import (
	"context"

	. "github.com/stealthrocket/wazergo/types"
)

func (m *Module) errorIsTemporary(ctx context.Context, err Int32) Bool {
	return Bool(errorValueOf(err).temporary)
}

func (m *Module) errorIsTimeout(ctx context.Context, err Int32) Bool {
	return Bool(errorValueOf(err).timeout)
}

func (m *Module) errorMessageSize(ctx context.Context, err Int32) Int32 {
	return Int32(len(errorValueOf(err).message))
}

func (m *Module) errorMessageRead(ctx context.Context, err Int32, buf Bytes) Int32 {
	return Int32(copy(buf, errorValueOf(err).message))
}

const (
	errNone = iota
	errInvalidArgument
	errTimeout
)

type errorValue struct {
	message   string
	temporary bool
	timeout   bool
}

var errorValues = [...]errorValue{
	errNone: {
		message: "(none)",
	},

	errInvalidArgument: {
		message: "invalid argument",
	},

	errTimeout: {
		message:   "timeout",
		temporary: true,
		timeout:   true,
	},
}

func errorValueOf(err Int32) errorValue {
	if err < Int32(len(errorValues)) {
		return errorValues[err]
	}
	return errorValue{message: "unknown runtime error"}
}
