package timemachine

import (
	"context"
	"fmt"
	"time"
	"unsafe"

	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero/api"
)

// Capture is a decorator that captures details about host function calls.
func Capture[T wazergo.Module](functions FunctionIndex, capture func(Record)) wazergo.Decorator[T] {
	return wazergo.DecoratorFunc[T](func(moduleName string, f wazergo.Function[T]) wazergo.Function[T] {
		functionID, ok := functions.Lookup(moduleName, f.Name)
		if !ok {
			return f
		}
		return wazergo.Function[T]{
			Name:    f.Name,
			Params:  f.Params,
			Results: f.Results,
			Func: func(module T, ctx context.Context, mod api.Module, stack []uint64) {
				record := Record{
					Timestamp: time.Now(),
					Function:  functionID,
				}

				if bufferSize := int(unsafe.Sizeof(record.Stack) / unsafe.Sizeof(record.Stack[0])); bufferSize < len(f.Params)+len(f.Results) {
					panic(fmt.Sprintf("record.Stack (%d) is not large enough to hold params (%d) and results (%d) for %s.%s", bufferSize, len(f.Params), len(f.Results), moduleName, f.Name))
				}
				record.Params = record.Stack[:len(f.Params)]
				record.Results = record.Stack[len(f.Params) : len(f.Params)+len(f.Results)]
				copy(record.Params, stack[:len(f.Params)])

				memoryInterceptorModule := GetMemoryInterceptorModule(mod)
				defer PutMemoryInterceptorModule(memoryInterceptorModule)

				defer func() {
					record.MemoryAccess = memoryInterceptorModule.Memory().(*MemoryInterceptor).MemoryAccess()

					copy(record.Results, stack[:len(f.Results)])

					capture(record)
				}()

				f.Func(module, ctx, memoryInterceptorModule, stack)
			},
		}
	})
}
