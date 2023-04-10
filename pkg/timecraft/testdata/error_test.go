package testdata

import (
	"runtime/timecraft"
	"testing"
)

// A few well-known error codes, keep a copy here so we can detect changes if
// the error codes are updated in the runtime.
const (
	errNone timecraft.Error = iota
	errInvalidArgument
	errTimeout
)

func TestErrorIsTemporary(t *testing.T) {
	if errInvalidArgument.Temporary() {
		t.Error("invalid argument error is temporary")
	}
	if !errTimeout.Temporary() {
		t.Error("timeout error is not temporary")
	}
}

func TestErrorIsTimeout(t *testing.T) {
	if errInvalidArgument.Timeout() {
		t.Error("invalid argument error is a timeout")
	}
	if !errTimeout.Timeout() {
		t.Error("timeout error is not a timeout")
	}
}
