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

				f.Func(module, ctx, memoryInterceptorModule, stack)

				record.MemoryAccess = memoryInterceptorModule.Memory().(*MemoryInterceptor).Mutations()

				copy(record.Results, stack[:len(f.Results)])

				capture(record)
			},
		}
	})
}

type memoryCaptureModule struct {
	api.Module
	mem *MemoryInterceptor
}

func (m *memoryCaptureModule) Memory() api.Memory {
	return m.mem
}

// FunctionIndex is a set of functions.
type FunctionIndex struct {
	lookup    map[Function]int
	functions []Function
}

// Add adds a function to the set.
func (i *FunctionIndex) Add(moduleName, functionName string) bool {
	if i.lookup == nil {
		i.lookup = map[Function]int{}
	}
	fn := Function{moduleName, functionName}
	if _, exists := i.lookup[fn]; exists {
		return false
	}
	i.lookup[fn] = len(i.functions)
	i.functions = append(i.functions, fn)
	return true
}

// Lookup returns the ID associated with a function.
func (i *FunctionIndex) Lookup(moduleName, functionName string) (int, bool) {
	fn := Function{Module: moduleName, Name: functionName}
	id, ok := i.lookup[fn]
	return id, ok
}

// Functions is the set of functions.
func (i *FunctionIndex) Functions() []Function {
	return i.functions
}
