package textprint

import (
	"cmp"
	"fmt"
	"io"
	"reflect"
	"slices"
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
	cmpFunc := cmpFuncOf(key)
	encodeKey := encodeFuncOf(key)
	encodeVal := encodeFuncOf(val)
	return func(w io.Writer, v reflect.Value) error {
		keys := v.MapKeys()
		slices.SortFunc(keys, cmpFunc)

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

type cmpFunc func(reflect.Value, reflect.Value) int

func cmpBool(v1, v2 reflect.Value) int {
	b1 := v1.Bool()
	b2 := v2.Bool()
	if !b1 && b2 {
		return -1
	} else if b1 && !b2 {
		return 1
	}
	return 0
}

func cmpInt(v1, v2 reflect.Value) int {
	return cmp.Compare(v1.Int(), v2.Int())
}

func cmpUint(v1, v2 reflect.Value) int {
	return cmp.Compare(v1.Uint(), v2.Uint())
}

func cmpString(v1, v2 reflect.Value) int {
	return cmp.Compare(v1.String(), v2.String())
}

func cmpFuncOf(t reflect.Type) cmpFunc {
	switch t.Kind() {
	case reflect.Bool:
		return cmpBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return cmpInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return cmpUint
	case reflect.String:
		return cmpString
	default:
		panic("cannot compare values of type " + t.String())
	}
}
