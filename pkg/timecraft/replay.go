package timecraft

import (
	"bytes"
	"context"
	"fmt"

	"github.com/stealthrocket/plugins/wasm"
	"github.com/tetratelabs/wazero/api"
	"golang.org/x/exp/slices"
)

func Replay[T wasm.Module](playback func() *Record) wasm.Decorator[T] {
	return wasm.DecoratorFunc[T](func(moduleName string, fn wasm.Function[T]) wasm.Function[T] {
		if playback == nil {
			return fn
		}
		f := fn.Func

		fn.Func = func(this T, ctx context.Context, module api.Module, stack []uint64) {
			rec := playback()

			if rec == nil {
				panic(fmt.Errorf("unexpected function call: %s::%s", moduleName, fn.Name))
			}

			if moduleName != rec.Module || fn.Name != rec.Function {
				panic(fmt.Errorf("unexpected function call: want=%s::%s got=%s::%s", rec.Module, rec.Function, moduleName, fn.Name))
			}

			if len(stack) < len(rec.Params) {
				panic(fmt.Errorf("stack too short to hold parameters: want=%d got=%d", len(rec.Params), len(stack)))
			}

			if len(stack) < len(rec.Results) {
				panic(fmt.Errorf("stack too short to hold results: want=%d got=%d", len(rec.Results), len(stack)))
			}

			if !slices.Equal(stack[:len(rec.Params)], rec.Params) {
				panic(fmt.Errorf("unexpected parameters: want=%v got=%v", rec.Params, stack[:len(rec.Params)]))
			}

			memory := module.Memory()

			for _, mem := range rec.Memory {
				b, ok := memory.Read(mem.Offset, mem.Length)
				if !ok {
					panic(wasm.SEGFAULT{Offset: mem.Offset, Length: mem.Length})
				}
				if mem.Write {
					copy(b, mem.Bytes)
				} else if !bytes.Equal(b, mem.Bytes) {
					panic(fmt.Errorf("unexpected memory content at offset %d: want=%q got=%q", mem.Offset, mem.Bytes, b))
				}
			}

			if moduleName == "wasi_snapshot_preview1" && fn.Name == "fd_write" && (stack[0] == 1 || stack[0] == 2) {
				// HACK: let writes to stdout/stderr through so we can debug
				f(this, ctx, module, stack)
			}

			copy(stack, rec.Results)
		}
		return fn
	})
}
