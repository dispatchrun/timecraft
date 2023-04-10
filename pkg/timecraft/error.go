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
	errNone Int32 = iota
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
}

func errorValueOf(err Int32) errorValue {
	if err < Int32(len(errorValues)) {
		return errorValues[err]
	}
	return errorValue{message: "unknown runtime error"}
}
