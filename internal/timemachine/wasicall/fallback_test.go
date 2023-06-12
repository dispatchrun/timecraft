package wasicall

import (
	"context"
	"testing"

	"github.com/stealthrocket/wasi-go"
)

func TestFallback(t *testing.T) {
	const exitCode = 23

	t.Run("call primary", func(t *testing.T) {
		system := NewFallbackSystem(NewErrnoSystem(wasi.ESUCCESS), NewExitSystem(exitCode))
		for _, syscall := range syscalls {
			call(context.Background(), system, syscall)
		}
	})

	t.Run("call fallback", func(t *testing.T) {
		system := NewFallbackSystem(NewErrnoSystem(wasi.ENOSYS), NewErrnoSystem(wasi.ESUCCESS))
		for _, syscall := range syscalls {
			if res := call(context.Background(), system, syscall); res.Error() != wasi.ESUCCESS {
				t.Errorf("unexpected result: got %v, expect %v", res.Error(), wasi.ESUCCESS)
			}
		}
	})

	t.Run("replay through primary", func(t *testing.T) {
		for _, syscall := range syscalls {
			t.Run(syscallString(syscall), func(t *testing.T) {
				defer suppressProcExit(syscall)

				testReplay(t, syscall, func(replay wasi.System) wasi.System {
					return NewFallbackSystem(replay, NewExitSystem(exitCode))
				})
			})
		}
	})

	t.Run("replay through fallback", func(t *testing.T) {
		for _, syscall := range syscalls {
			t.Run(syscallString(syscall), func(t *testing.T) {
				defer suppressProcExit(syscall)

				testReplay(t, syscall, func(replay wasi.System) wasi.System {
					return NewFallbackSystem(NewErrnoSystem(wasi.ENOSYS), replay)
				})
			})
		}
	})
}
