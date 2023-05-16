package timemachine

import (
	"context"
	"time"

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
				stackCopy := make([]uint64, 0, len(f.Params)+len(f.Results))
				params, results := stackCopy[:len(f.Params)], stackCopy[len(f.Params):len(f.Params)+len(f.Results)]
				copy(params, stack[:len(f.Params)])

				record := Record{
					Timestamp: time.Now(),
					Function:  functionID,
					Params:    params,
					Results:   results,
				}

				f.Func(module, ctx, mod, stack)

				copy(results, stack[:len(f.Results)])

				capture(record)
			},
		}
	})
}

type memoryCaptureModule struct {
	api.Module
	mem memoryCapture
}

func (m *memoryCaptureModule) Memory() api.Memory {
	return m.mem
}

type memoryCapture struct {
	api.Memory
}

/* TODO: capture memory mutations
Read(offset, byteCount uint32) ([]byte, bool)
WriteByte(offset uint32, v byte) bool
WriteUint16Le(offset uint32, v uint16) bool
WriteUint32Le(offset, v uint32) bool
WriteFloat32Le(offset uint32, v float32) bool
WriteUint64Le(offset uint32, v uint64) bool
WriteFloat64Le(offset uint32, v float64) bool
Write(offset uint32, v []byte) bool
WriteString(offset uint32, v string) bool
*/

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
