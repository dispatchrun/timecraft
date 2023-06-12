package wasicall

import (
	"context"
	"testing"

	"github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero/sys"
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
		system := NewFallbackSystem(NewErrnoSystem(wasi.ENOSYS), NewExitSystem(exitCode))
		for _, syscall := range syscalls {
			func() {
				defer func() {
					if err := recover(); err != nil {
						ee, ok := err.(*sys.ExitError)
						if !ok || ee.ExitCode() != exitCode {
							panic(err)
						}
					}
				}()

				call(context.Background(), system, syscall)

				t.Fatal("expected the fallback system to be taken")
			}()
		}
	})

	t.Run("replay through primary", func(t *testing.T) {
		for _, syscall := range syscalls {
			testReplay(t, syscall, func(replay wasi.System) wasi.System {
				return NewFallbackSystem(replay, NewExitSystem(exitCode))
			})
		}
	})

	t.Run("replay through fallback", func(t *testing.T) {
		for _, syscall := range syscalls {
			testReplay(t, syscall, func(replay wasi.System) wasi.System {
				return NewFallbackSystem(NewErrnoSystem(wasi.ENOSYS), replay)
			})
		}
	})
}
