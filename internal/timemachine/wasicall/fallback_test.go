package wasicall

import (
	"context"
	"testing"

	"github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero/sys"
)

func TestFallback(t *testing.T) {
	const exitCode = 23

	t.Run("no fallback", func(t *testing.T) {
		system := NewFallbackSystem(errnoSystem(wasi.ESUCCESS), &exitSystem{exitCode})
		for _, syscall := range validSyscalls {
			pushParams(context.Background(), syscall, system)
		}
	})

	t.Run("fallback", func(t *testing.T) {
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

				pushParams(context.Background(), syscall, system)

				t.Fatal("expected the fallback system to be taken")
			}()
		}
	})

	// TODO: check that the fallback system forwards args, and returns values, correctly
	// TODO: check the case where either or both of the systems don't implement SocketsExtension
}
