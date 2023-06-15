package timecraft

import (
	"encoding/json"

	"gopkg.in/yaml.v3"
)

type Nullable[T any] struct {
	value T
	exist bool
}

// Note: commented to satisfy the linter, uncomment if we need it
//
// func null[T any]() Nullable[T] {
// 	return Nullable[T]{exist: false}
// }

func NullableValue[T any](v T) Nullable[T] {
	return Nullable[T]{value: v, exist: true}
}

func (v Nullable[T]) Value() (T, bool) {
	return v.value, v.exist
}

func (v Nullable[T]) MarshalJSON() ([]byte, error) {
	if !v.exist {
		return []byte("null"), nil
	}
	return json.Marshal(v.value)
}

func (v Nullable[T]) MarshalYAML() (any, error) {
	if !v.exist {
		return nil, nil
	}
	return v.value, nil
}

func (v *Nullable[T]) UnmarshalJSON(b []byte) error {
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

func (v *Nullable[T]) UnmarshalYAML(node *yaml.Node) error {
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
