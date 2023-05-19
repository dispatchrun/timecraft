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
		var paramCount int
		for _, v := range f.Params {
			paramCount += len(v.ValueTypes())
		}
		var resultCount int
		for _, v := range f.Results {
			resultCount += len(v.ValueTypes())
		}
		functionID, ok := functions.Lookup(Function{
			Module:      moduleName,
			Name:        f.Name,
			ParamCount:  paramCount,
			ResultCount: resultCount,
		})
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

				if bufferSize := int(unsafe.Sizeof(record.Stack) / unsafe.Sizeof(record.Stack[0])); bufferSize < paramCount+resultCount {
					panic(fmt.Sprintf("record.Stack (%d) is not large enough to hold params (%d) and results (%d) for %s.%s", bufferSize, paramCount, resultCount, moduleName, f.Name))
				}
				record.Params = record.Stack[:paramCount]
				record.Results = record.Stack[paramCount : paramCount+resultCount]
				copy(record.Params, stack[:paramCount])

				memoryInterceptorModule := GetMemoryInterceptorModule(mod)
				defer PutMemoryInterceptorModule(memoryInterceptorModule)

				defer func() {
					record.MemoryAccess = memoryInterceptorModule.Memory().(*MemoryInterceptor).MemoryAccess()

					copy(record.Results, stack[:resultCount])

					capture(record)
				}()

				f.Func(module, ctx, memoryInterceptorModule, stack)
			},
		}
	})
}
