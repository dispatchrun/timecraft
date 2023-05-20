package functioncall

import (
	"context"
	"time"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero/api"
)

// Record is a decorator that records details about host function calls.
func Record[T wazergo.Module](startTime time.Time, functions timemachine.FunctionIndex, record func(timemachine.RecordBuilder)) wazergo.Decorator[T] {
	var interceptor MemoryInterceptorModule
	var functionCallBuilder FunctionCallBuilder
	var recordBuilder timemachine.RecordBuilder

	return wazergo.DecoratorFunc[T](func(moduleName string, original wazergo.Function[T]) wazergo.Function[T] {
		function := timemachine.Function{
			Module:      moduleName,
			Name:        original.Name,
			ParamCount:  original.NumParams(),
			ResultCount: original.NumResults(),
		}
		functionID, ok := functions.LookupFunction(function)
		if !ok {
			return original
		}
		return original.WithFunc(func(module T, ctx context.Context, mod api.Module, stack []uint64) {
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

				record(recordBuilder)
			}()

			original.Func(module, ctx, &interceptor, stack)
		})
	})
}
