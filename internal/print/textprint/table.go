package textprint

import (
	"fmt"
	"io"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/stealthrocket/timecraft/internal/stream"
)

func NewTableWriter[T any](w io.Writer) stream.WriteCloser[T] {
	t := &tableWriter[T]{
		writer: tabwriter.NewWriter(w, 0, 4, 2, ' ', 0),
	}

	writeString := func(w io.Writer, s string) {
		_, err := io.WriteString(w, s)
		if err != nil {
			panic(err)
		}
	}

	var v T
	for i, f := range reflect.VisibleFields(reflect.TypeOf(v)) {
		if i != 0 {
			writeString(t.writer, "\t")
		}

		name := f.Name
		if tag := strings.Split(f.Tag.Get("text"), ","); len(tag) > 0 {
			name, tag = tag[0], tag[1:]
			for _, s := range tag {
				switch s {
				// TODO: other tags
				}
			}
		}

		writeString(t.writer, name)
		t.encoders = append(t.encoders, encodeFuncOfStructField(f.Type, f.Index))
	}

	writeString(t.writer, "\n")
	return t
}

type tableWriter[T any] struct {
	writer   *tabwriter.Writer
	encoders []encodeFunc
}

func (t *tableWriter[T]) Write(values []T) (int, error) {
	for n := range values {
		v := reflect.ValueOf(&values[n]).Elem()
		w := io.Writer(t.writer)

		for i, enc := range t.encoders {
			if i != 0 {
				_, err := io.WriteString(w, "\t")
				if err != nil {
					return n, err
				}
			}
			if err := enc(w, v); err != nil {
				return n, err
			}
		}

		if _, err := io.WriteString(w, "\n"); err != nil {
			return n, err
		}
	}
	return len(values), nil
}

func (t *tableWriter[T]) Close() error {
	return t.writer.Flush()
}

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

func encodeFuncOf(t reflect.Type) encodeFunc {
	switch t.Kind() {
	case reflect.Bool:
		return encodeBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return encodeInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint64, reflect.Uintptr:
		return encodeUint
	case reflect.String:
		return encodeString
	case reflect.Pointer:
		return encodeFuncOfPointer(t.Elem())
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

func encodeFuncOfStructField(t reflect.Type, index []int) encodeFunc {
	encode := encodeFuncOf(t)
	return func(w io.Writer, v reflect.Value) error {
		return encode(w, v.FieldByIndex(index))
	}
}
