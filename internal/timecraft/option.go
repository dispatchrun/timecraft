package timecraft

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

// Option is a value of type T or null.
type Option[T any] struct {
	value T
	exist bool
}

// Some creates an Option with a value.
func Some[T any](v T) Option[T] {
	return Option[T]{value: v, exist: true}
}

// Note: commented to satisfy the linter, uncomment if we need it
// func None[T any]() (_ Option[T]) {
// 	return
// }

// Value retrieves the value and a null flag from the Option.
func (v Option[T]) Value() (T, bool) {
	return v.value, v.exist
}

func (v Option[T]) MarshalJSON() ([]byte, error) {
	if !v.exist {
		return []byte("null"), nil
	}
	return json.Marshal(v.value)
}

func (v Option[T]) MarshalYAML() (any, error) {
	if !v.exist {
		return nil, nil
	}
	return v.value, nil
}

func (v *Option[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		v.exist = false
		return nil
	} else if err := json.Unmarshal(b, &v.value); err != nil {
		v.exist = false
		return err
	} else {
		v.exist = true
		return nil
	}
}

func (v *Option[T]) UnmarshalYAML(node *yaml.Node) error {
	if node.Value == "" || node.Value == "~" || node.Value == "null" {
		v.exist = false
		return nil
	} else if err := node.Decode(&v.value); err != nil {
		v.exist = false
		return err
	} else {
		v.exist = true
		return nil
	}
}
