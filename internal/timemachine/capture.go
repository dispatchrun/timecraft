package timemachine

import (
	"context"
	"time"

	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero/api"
)

// Capture is a decorator that captures details about host function calls.
func Capture[T wazergo.Module](startTime time.Time, functions FunctionIndex, capture func(RecordBuilder)) wazergo.Decorator[T] {
	var interceptor MemoryInterceptorModule
	var functionCallBuilder FunctionCallBuilder
	var recordBuilder RecordBuilder

	return wazergo.DecoratorFunc[T](func(moduleName string, f wazergo.Function[T]) wazergo.Function[T] {
		function := Function{
			Module:      moduleName,
			Name:        f.Name,
			ParamCount:  f.StackParamCount(),
			ResultCount: f.StackResultCount(),
		}
		functionID, ok := functions.LookupFunction(function)
		if !ok {
			return f
		}
		return wazergo.Decorated(f, func(module T, ctx context.Context, mod api.Module, stack []uint64) {
			now := time.Now()

			functionCallBuilder.Reset(&function)
			interceptor.Reset(mod, functionCallBuilder.MemoryInterceptor(mod.Memory()))
			functionCallBuilder.SetParams(stack[:function.ParamCount])

			defer func() {
				functionCallBuilder.SetResults(stack[:function.ResultCount])

				recordBuilder.Reset(startTime)
				recordBuilder.SetTimestamp(now)
				recordBuilder.SetFunctionID(functionID)
				recordBuilder.SetFunctionCall(functionCallBuilder.Bytes())

				capture(recordBuilder)
			}()

			f.Func(module, ctx, &interceptor, stack)
		})
	})
}
