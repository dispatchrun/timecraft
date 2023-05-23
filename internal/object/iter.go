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

// Empty constructs an empty iterator over items of type T.
func Empty[T any]() Iter[T] {
	return emptyIter[T]{}
}

type emptyIter[T any] struct{}

func (emptyIter[T]) Close() error { return nil }
func (emptyIter[T]) Next() bool   { return false }
func (emptyIter[T]) Item() (_ T)  { return }

// Err constructs an iterator over items of type T which produces no items and
// returns the given error when closed.
func Err[T any](err error) Iter[T] {
	return &errorIter[T]{err: err}
}

type errorIter[T any] struct{ err error }

func (it *errorIter[T]) Close() error { return it.err }
func (it *errorIter[T]) Next() bool   { return false }
func (it *errorIter[T]) Item() (_ T)  { return }
