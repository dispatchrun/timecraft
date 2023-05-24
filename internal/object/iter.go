package object

// Iter is a generic interface representing an iterator over a sequence of items
// of type T.
//
// The following snippet illustrate a typical use of iterators:
//
//	iter := ...
//	for iter.Next() {
//		item := iter.Item()
//		...
//	}
//	if err := iter.Close(); err != nil {
//		...
//	}
//
// Errors that occur during the iterator are reported as the value returned when
// closing the iterator.
type Iter[T any] interface {
	// Closes the iterator. Calling this method causes all future calls to Next
	// to return false, and returns any error encountered while iterating over
	// the sequence of items.
	Close() error

	// Advances the iterator to the next item. When initialized, the iterator is
	// positionned before the first item, so the method must be called prior to
	// calling Item.
	Next() bool

	// Returns the item that the iterator is currently positioned on.
	Item() T
}

// Items reads all items from the given iterator and returns them as a slice.
//
// The function closes the iterator prior to returning.
func Items[T any](iter Iter[T]) ([]T, error) {
	var items []T
	for iter.Next() {
		items = append(items, iter.Item())
	}
	return items, iter.Close()
}

// EmptyIter constructs an empty iterator over items of type T.
func EmptyIter[T any]() Iter[T] {
	return emptyIter[T]{}
}

type emptyIter[T any] struct{}

func (emptyIter[T]) Close() error { return nil }
func (emptyIter[T]) Next() bool   { return false }
func (emptyIter[T]) Item() (_ T)  { return }

// ErrorIter constructs an iterator over items of type T which produces no items
// and returns the given error when closed.
func ErrorIter[T any](err error) Iter[T] {
	return &errorIter[T]{err: err}
}

type errorIter[T any] struct{ err error }

func (it *errorIter[T]) Close() error { return it.err }
func (it *errorIter[T]) Next() bool   { return false }
func (it *errorIter[T]) Item() (_ T)  { return }

// ConvertIter converts a sequence of items of type From to a sequence of items
// of type To, using the conversion function passed as argument.
func ConvertIter[To, From any](base Iter[From], conv func(From) (To, error)) Iter[To] {
	return &convertIter[To, From]{base: base, conv: conv}
}

type convertIter[To, From any] struct {
	base Iter[From]
	conv func(From) (To, error)
	item To
	err  error
}

func (it *convertIter[To, From]) Close() error {
	if err := it.base.Close(); err != nil {
		return err
	}
	return it.err
}

func (it *convertIter[To, From]) Next() bool {
	if it.err != nil {
		return false
	}
	if !it.base.Next() {
		return false
	}
	if item, err := it.conv(it.base.Item()); err != nil {
		it.err = err
		return false
	} else {
		it.item = item
		return true
	}
}

func (it *convertIter[To, From]) Item() To {
	return it.item
}
