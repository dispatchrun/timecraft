package timemachine

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

// Lookup returns the ID associated with a function.
func (i *FunctionIndex) Lookup(fn Function) (int, bool) {
	id, ok := i.lookup[fn]
	return id, ok
}

// Functions is the set of functions.
func (i *FunctionIndex) Functions() []Function {
	return i.functions
}
