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
		system := NewFallbackSystem(errnoSystem(wasi.ESUCCESS), &exitSystem{exitCode})
		for _, syscall := range syscalls {
			call(context.Background(), system, syscall)
		}
	})

	t.Run("call fallback", func(t *testing.T) {
		system := NewFallbackSystem(errnoSystem(wasi.ENOSYS), &exitSystem{exitCode})
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
			testReplay(t, syscall, func(replay SocketsSystem) SocketsSystem {
				return NewFallbackSystem(replay, &exitSystem{exitCode})
			})
		}
	})

	t.Run("replay through fallback", func(t *testing.T) {
		for _, syscall := range syscalls {
			testReplay(t, syscall, func(replay SocketsSystem) SocketsSystem {
				return NewFallbackSystem(errnoSystem(wasi.ENOSYS), replay)
			})
		}
	})
}
