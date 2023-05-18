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
				// TODO: better error handling
				if !records.Next() {
					if err := records.Error(); err != nil {
						panic(err)
					}
					panic("EOF")
				}
				record := records.Record()
				if recordFunctionID := record.FunctionId(); recordFunctionID != functionID {
					panic(fmt.Sprintf("function ID mismatch: got %d, expect %d", functionID, recordFunctionID))
				}
				if len(f.Params) != record.NumParams() {
					panic(fmt.Sprintf("function param count mismatch: got %d, expect %d", len(f.Params), record.NumParams()))
				}
				for i := 0; i < record.NumParams(); i++ {
					if recordParam := record.ParamAt(i); recordParam != stack[i] {
						panic(fmt.Sprintf("function param %d mismatch: got %d, expect %d", i, stack[i], recordParam))
					}
				}
				if len(f.Results) != record.NumResults() {
					panic(fmt.Sprintf("function result count mismatch: got %d, expect %d", len(f.Results), record.NumResults()))
				}

				memory := mod.Memory()
				for i := 0; i < record.NumMemoryAccess(); i++ {
					m := record.MemoryAccessAt(i)
					b, ok := memory.Read(m.Offset, uint32(len(m.Memory)))
					if !ok {
						panic(fmt.Sprintf("unable to write %d bytes of memory to offset %d", len(m.Memory), m.Offset))
					}
					copy(b, m.Memory)
				}

				for i := 0; i < record.NumResults(); i++ {
					stack[i] = record.ResultAt(i)
				}

				// TODO: the record doesn't capture the fact that a host function
				//  didn't return. Hard-code this case for now.
				if moduleName == wasi_snapshot_preview1.HostModuleName && f.Name == "proc_exit" {
					panic(sys.NewExitError(uint32(stack[0])))
				}
			},
		}
	})
}
