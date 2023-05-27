package textprint

import (
	"io"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/stealthrocket/timecraft/internal/stream"
)

func NewTableWriter[T any](w io.Writer) stream.WriteCloser[T] {
	t := &tableWriter[T]{
		writer: tabwriter.NewWriter(w, 0, 4, 2, ' ', 0),
		valueOf: func(values []T, index int) reflect.Value {
			return reflect.ValueOf(&values[index]).Elem()
		},
	}

	writeString := func(w io.Writer, s string) {
		_, err := io.WriteString(w, s)
		if err != nil {
			panic(err)
		}
	}

	var v T
	valueType := reflect.TypeOf(v)
	if valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
		t.valueOf = func(values []T, index int) reflect.Value {
			return reflect.ValueOf(values[index]).Elem()
		}
	}

	for i, f := range reflect.VisibleFields(valueType) {
		if i != 0 {
			writeString(t.writer, "\t")
		}

		name := f.Name
		if textTag := f.Tag.Get("text"); textTag != "" {
			tag := strings.Split(textTag, ",")
			name, tag = tag[0], tag[1:]
			for _, s := range tag {
				switch s {
				// TODO: other tags
				}
			}
		}

		if name == "-" {
			continue
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
	valueOf  func([]T, int) reflect.Value
}

func (t *tableWriter[T]) Write(values []T) (int, error) {
	for n := range values {
		v := t.valueOf(values, n)
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
