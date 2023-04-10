package timecraft_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stealthrocket/timecraft/pkg/timecraft"
)

func TestTimecraft(t *testing.T) {
	files, _ := filepath.Glob("testdata/*.wasm")

	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			bytecode, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()

			process, err := timecraft.SpawnProcess(ctx, bytecode)
			if err != nil {
				t.Fatal(err)
			}
			defer process.Close(ctx)

			if err := process.Run(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}
