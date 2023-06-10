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
		for _, syscall := range validSyscalls {
			call(context.Background(), system, syscall)
		}
	})

	t.Run("call fallback", func(t *testing.T) {
		system := NewFallbackSystem(errnoSystem(wasi.ENOSYS), &exitSystem{exitCode})
		for _, syscall := range validSyscalls {
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

	// TODO: check that the fallback system forwards args, and returns values, correctly
	// TODO: check the case where either or both of the systems don't implement SocketsExtension
}
