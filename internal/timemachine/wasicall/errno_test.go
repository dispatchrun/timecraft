package wasicall

import (
	"context"
	"testing"

	"github.com/stealthrocket/wasi-go"
)

func TestErnnoSystem(t *testing.T) {
	const errno = wasi.ENOSYS

	system := NewErrnoSystem(errno)

	for _, syscall := range syscalls {
		t.Run(syscallString(syscall), func(t *testing.T) {
			if res := call(context.Background(), system, syscall); res.Error() != errno {
				t.Fatalf("unexpected errno: got %v, expect %v", res.Error(), errno)
			}
		})
	}
}
