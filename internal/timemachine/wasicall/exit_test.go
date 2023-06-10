package wasicall

import (
	"context"
	"testing"

	"github.com/tetratelabs/wazero/sys"
)

func TestExitSystem(t *testing.T) {
	const exitCode = 23

	system := &exitSystem{exitCode}

	for _, syscall := range validSyscalls {
		t.Run(syscallString(syscall), func(t *testing.T) {
			defer func() {
				if err := recover(); err != nil {
					if ee, ok := err.(*sys.ExitError); !ok || ee.ExitCode() != exitCode {
						panic(err)
					}
				}
			}()

			pushParams(context.Background(), syscall, system)

			t.Fatal("expected panic")
		})
	}
}
