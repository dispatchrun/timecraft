package query

import "time"

type Value interface {
	After(time.Time) bool
	Before(time.Time) bool
	Match(name, value string) bool
}

type Filter[T Value] interface {
	Match(T) bool
}

type After[T Value] time.Time

func (op After[T]) Match(value T) bool {
	return value.After(time.Time(op))
}

type Before[T Value] time.Time

func (op Before[T]) Match(value T) bool {
	return value.Before(time.Time(op))
}

type Match[T Value] [2]string

func (op Match[T]) Match(value T) bool {
	return value.Match(op[0], op[1])
}

type And[T Value] [2]Filter[T]

func (op And[T]) Match(value T) bool {
	return op[0].Match(value) && op[1].Match(value)
}

type Or[T Value] [2]Filter[T]

func (op Or[T]) Match(value T) bool {
	return op[0].Match(value) || op[1].Match(value)
}

type Not[T Value] [1]Filter[T]

func (op Not[T]) Match(value T) bool {
	return !op[0].Match(value)
}

func MatchAll[T Value](value T, filters ...Filter[T]) bool {
	for _, filter := range filters {
		if !filter.Match(value) {
			return false
		}
	}
	return true
}

func MatchOne[T Value](value T, filters ...Filter[T]) bool {
	for _, filter := range filters {
		if filter.Match(value) {
			return true
		}
	}
	return false
}
