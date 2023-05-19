package timemachine

import (
	"context"
	"fmt"

	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
)

func Replay[T wazergo.Module](functions FunctionIndex, records *LogRecordIterator) wazergo.Decorator[T] {
	return wazergo.DecoratorFunc[T](func(moduleName string, original wazergo.Function[T]) wazergo.Function[T] {
		function := Function{
			Module:      moduleName,
			Name:        original.Name,
			ParamCount:  original.StackParamCount(),
			ResultCount: original.StackResultCount(),
		}
		functionID, ok := functions.LookupFunction(function)
		if !ok {
			return original
		}
		return original.WithFunc(func(module T, ctx context.Context, mod api.Module, stack []uint64) {
			// TODO: better error handling
			if !records.Next() {
				panic("EOF")
			}
			record, err := records.Record()
			if err != nil {
				panic(err)
			}
			if recordFunctionID := record.FunctionID(); recordFunctionID != functionID {
				panic(fmt.Sprintf("function ID mismatch: got %d, expect %d", functionID, recordFunctionID))
			}
			functionCall, err := record.FunctionCall()
			if err != nil {
				panic(err)
			}
			if function.ParamCount != functionCall.NumParams() {
				panic(fmt.Sprintf("function param count mismatch: got %d, expect %d", function.ParamCount, functionCall.NumParams()))
			}
			for i := 0; i < function.ParamCount; i++ {
				if param := functionCall.Param(i); param != stack[i] {
					panic(fmt.Sprintf("function param %d mismatch: got %d, expect %d", i, stack[i], param))
				}
			}

			memory := mod.Memory()
			for i := 0; i < functionCall.NumMemoryAccess(); i++ {
				m := functionCall.MemoryAccess(i)
				b, ok := memory.Read(m.Offset, uint32(len(m.Memory)))
				if !ok {
					panic(fmt.Sprintf("unable to write %d bytes of memory to offset %d", len(m.Memory), m.Offset))
				}
				copy(b, m.Memory)
			}

			// TODO: the record doesn't capture the fact that a host function
			//  didn't return. Hard-code this case for now.
			if moduleName == wasi_snapshot_preview1.HostModuleName && function.Name == "proc_exit" {
				panic(sys.NewExitError(uint32(stack[0])))
			}

			if function.ResultCount != functionCall.NumResults() {
				panic(fmt.Sprintf("function result count mismatch: got %d, expect %d", function.ResultCount, functionCall.NumResults()))
			}
			for i := 0; i < function.ResultCount; i++ {
				stack[i] = functionCall.Result(i)
			}
		})
	})
}
