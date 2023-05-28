package textprint

import (
	"fmt"
	"io"
	"reflect"

	"golang.org/x/exp/slices"
)

type encodeFunc func(io.Writer, reflect.Value) error

func encodeBool(w io.Writer, v reflect.Value) error {
	_, err := fmt.Fprintf(w, "%t", v.Bool())
	return err
}

func encodeInt(w io.Writer, v reflect.Value) error {
	_, err := fmt.Fprintf(w, "%d", v.Int())
	return err
}

func encodeUint(w io.Writer, v reflect.Value) error {
	_, err := fmt.Fprintf(w, "%d", v.Uint())
	return err
}

func encodeString(w io.Writer, v reflect.Value) error {
	_, err := io.WriteString(w, v.String())
	return err
}

func encodeStringer(w io.Writer, v reflect.Value) error {
	_, err := io.WriteString(w, v.Interface().(fmt.Stringer).String())
	return err
}

func encodeFormatter(w io.Writer, v reflect.Value) error {
	_, err := fmt.Fprintf(w, "%v", v.Interface())
	return err
}

func encodeFuncOf(t reflect.Type) encodeFunc {
	if t.Implements(reflect.TypeOf((*fmt.Formatter)(nil)).Elem()) {
		return encodeFormatter
	}
	if t.Implements(reflect.TypeOf((*fmt.Stringer)(nil)).Elem()) {
		return encodeStringer
	}
	switch t.Kind() {
	case reflect.Bool:
		return encodeBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return encodeInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return encodeUint
	case reflect.String:
		return encodeString
	case reflect.Pointer:
		return encodeFuncOfPointer(t.Elem())
	case reflect.Slice:
		return encodeFuncOfSlice(t.Elem())
	case reflect.Map:
		return encodeFuncOfMap(t.Key(), t.Elem())
	default:
		panic("cannot encode values of type " + t.String())
	}
}

func encodeFuncOfPointer(t reflect.Type) encodeFunc {
	encode := encodeFuncOf(t)
	return func(w io.Writer, v reflect.Value) error {
		if v.IsNil() {
			_, err := fmt.Fprintf(w, "(none)")
			return err
		} else {
			return encode(w, v.Elem())
		}
	}
}

func encodeFuncOfSlice(t reflect.Type) encodeFunc {
	encode := encodeFuncOf(t)
	return func(w io.Writer, v reflect.Value) error {
		for i, n := 0, v.Len(); i < n; i++ {
			if i != 0 {
				if _, err := io.WriteString(w, ", "); err != nil {
					return err
				}
			}
			if err := encode(w, v.Index(i)); err != nil {
				return err
			}
		}
		return nil
	}
}

func encodeFuncOfMap(key, val reflect.Type) encodeFunc {
	lessFunc := lessFuncOf(key)
	encodeKey := encodeFuncOf(key)
	encodeVal := encodeFuncOf(val)
	return func(w io.Writer, v reflect.Value) error {
		keys := v.MapKeys()
		slices.SortFunc(keys, lessFunc)

		for i, key := range keys {
			if i != 0 {
				if _, err := io.WriteString(w, ", "); err != nil {
					return err
				}
			}
			if err := encodeKey(w, key); err != nil {
				return err
			}
			if _, err := io.WriteString(w, ":"); err != nil {
				return err
			}
			if err := encodeVal(w, v.MapIndex(key)); err != nil {
				return err
			}
		}

		return nil
	}
}

func encodeFuncOfStructField(t reflect.Type, index []int) encodeFunc {
	encode := encodeFuncOf(t)
	return func(w io.Writer, v reflect.Value) error {
		return encode(w, v.FieldByIndex(index))
	}
}

type lessFunc func(reflect.Value, reflect.Value) bool

func lessBool(v1, v2 reflect.Value) bool {
	b1 := v1.Bool()
	b2 := v2.Bool()
	return !b1 && b1 != b2
}

func lessInt(v1, v2 reflect.Value) bool {
	return v1.Int() < v2.Int()
}

func lessUint(v1, v2 reflect.Value) bool {
	return v1.Uint() < v2.Uint()
}

func lessString(v1, v2 reflect.Value) bool {
	return v1.String() < v2.String()
}

func lessFuncOf(t reflect.Type) lessFunc {
	switch t.Kind() {
	case reflect.Bool:
		return lessBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return lessInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return lessUint
	case reflect.String:
		return lessString
	default:
		panic("cannot compare values of type " + t.String())
	}
}
