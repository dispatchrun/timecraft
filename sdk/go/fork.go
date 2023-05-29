package fork

import (
	"context"
	"os"
)

//go:wasm-module env
//export fork
func fork() int32

// spanw creates a fork and only execute the child branch
func spawn(ctx context.Context, fn func(ctx context.Context) error) {
	if fork() == 0 {
		if err := fn(ctx); err != nil {
			os.Exit(-1) //TODO: better error handling
		}
	}
}
