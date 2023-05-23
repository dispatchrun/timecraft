package functioncall

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero/api"
)

// ReplayController controls a replay.
type ReplayController[T wazergo.Module] interface {
	// Step is called each time a function is called. If an error occurs,
	// one of the other hooks will be called instead.
	Step(ctx context.Context, module T, fn wazergo.Function[T], mod api.Module, stack []uint64, record *timemachine.Record)

	// ReadError is called when there's an error reading a record from
	// the log, or error parsing the record.
	ReadError(ctx context.Context, module T, fn wazergo.Function[T], mod api.Module, stack []uint64, err error)

	// MismatchError is called when a function is called that doesn't match
	// the next record.
	MismatchError(ctx context.Context, module T, fn wazergo.Function[T], mod api.Module, stack []uint64, record *timemachine.Record, err error)

	// Exit is called when the guest exits.
	Exit(ctx context.Context, module T, fn wazergo.Function[T], mod api.Module, stack []uint64, record *timemachine.Record, exitCode uint32)

	// EOF is called when a function is called and there are no more
	// logs to replay.
	EOF(ctx context.Context, module T, fn wazergo.Function[T], mod api.Module, stack []uint64)
}

// Replay is a decorator that replays host function calls recorded to a log.
func Replay[T wazergo.Module](functions timemachine.FunctionIndex, records *timemachine.LogRecordReader, controller ReplayController[T]) wazergo.Decorator[T] {
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
			record, err := records.ReadRecord()
			if err != nil {
				if errors.Is(err, io.EOF) {
					controller.EOF(ctx, module, original, mod, stack)
				} else {
					controller.ReadError(ctx, module, original, mod, stack, err)
				}
				return
			}
			recordFunction, err := record.Function()
			if err != nil {
				controller.ReadError(ctx, module, original, mod, stack, err)
				return
			}
			functionCall := MakeFunctionCall(recordFunction, record.FunctionCall())

			memory := mod.Memory()
			if err := assertEqual(functionID, function, stack, memory, record, functionCall); err != nil {
				controller.MismatchError(ctx, module, original, mod, stack, record, err)
				return
			}

			for i := 0; i < functionCall.NumMemoryAccess(); i++ {
				m := functionCall.MemoryAccess(i)
				b, ok := memory.Read(m.Offset, uint32(len(m.Memory)))
				if !ok {
					err = fmt.Errorf("out of bounds memory write [%d:%d]", m.Offset, len(m.Memory))
					controller.MismatchError(ctx, module, original, mod, stack, record, err)
					return
				}
				// TODO: if we could disambiguate reads/writes, we wouldn't have
				//  to unconditionally copy here (and do useless work when it
				//  was just a read)
				copy(b, m.Memory)
			}

			// TODO: the record doesn't capture the fact that a host function
			//  didn't return. Hard-code this case for now.
			if moduleName == wasi_snapshot_preview1.HostModuleName && function.Name == "proc_exit" {
				exitCode := uint32(stack[0])
				controller.Exit(ctx, module, original, mod, stack, record, exitCode)
				return
			}

			for i := 0; i < function.ResultCount; i++ {
				stack[i] = functionCall.Result(i)
			}
		})
	})
}

func assertEqual(functionID int, function timemachine.Function, stack []uint64, mem api.Memory, record *timemachine.Record, functionCall FunctionCall) error {
	if recordFunctionID := record.FunctionID(); recordFunctionID != functionID {
		return fmt.Errorf("function ID mismatch: got %d, expect %d", functionID, recordFunctionID)
	}
	if function.ParamCount != functionCall.NumParams() {
		return fmt.Errorf("function param count mismatch: got %d, expect %d", function.ParamCount, functionCall.NumParams())
	}
	if function.ResultCount != functionCall.NumResults() {
		return fmt.Errorf("function result count mismatch: got %d, expect %d", function.ResultCount, functionCall.NumResults())
	}
	for i := 0; i < function.ParamCount; i++ {
		if param := functionCall.Param(i); param != stack[i] {
			return fmt.Errorf("function param %d mismatch: got %d, expect %d", i, stack[i], param)
		}
	}
	// TODO: validate that memory is in the same state as captured
	//  memory reads (but not writes)
	return nil
}
