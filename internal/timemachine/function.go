package timemachine

// Function is a host function.
type Function struct {
	Module      string
	Name        string
	ParamCount  int
	ResultCount int
}

// FunctionIndex is a set of functions.
type FunctionIndex struct {
	lookup    map[Function]int
	functions []Function
}

// Add adds a function to the set.
func (i *FunctionIndex) Add(fn Function) bool {
	if i.lookup == nil {
		i.lookup = map[Function]int{}
	}
	if _, exists := i.lookup[fn]; exists {
		return false
	}
	i.lookup[fn] = len(i.functions)
	i.functions = append(i.functions, fn)
	return true
}

// Functions is the set of functions.
func (i *FunctionIndex) Functions() []Function {
	return i.functions
}

// LookupFunction returns the ID associated with a function.
func (i *FunctionIndex) LookupFunction(fn Function) (int, bool) {
	id, ok := i.lookup[fn]
	return id, ok
}

// LookupID returns the Function associated with an ID.
func (i *FunctionIndex) LookupID(id int) (Function, bool) {
	if id < 0 || id >= len(i.functions) {
		return Function{}, false
	}
	return i.functions[id], true
}
