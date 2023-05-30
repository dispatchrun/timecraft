package textprint

import (
	"io"
	"reflect"
	"strings"
	"text/tabwriter"

	"github.com/stealthrocket/timecraft/internal/stream"
	"golang.org/x/exp/slices"
)

type TableOption[T any] func(*tableWriter[T])

func OrderBy[T any](f func(T, T) bool) TableOption[T] {
	return func(t *tableWriter[T]) {
		t.orderBy = f
	}
}

func NewTableWriter[T any](w io.Writer, opts ...TableOption[T]) stream.WriteCloser[T] {
	t := &tableWriter[T]{
		output: w,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

type tableWriter[T any] struct {
	output  io.Writer
	values  []T
	orderBy func(T, T) bool
}

func (t *tableWriter[T]) Write(values []T) (int, error) {
	t.values = append(t.values, values...)
	return len(values), nil
}

func (t *tableWriter[T]) Close() error {
	tw := tabwriter.NewWriter(t.output, 0, 4, 2, ' ', 0)

	if t.orderBy != nil {
		slices.SortFunc(t.values, t.orderBy)
	}

	valueOf := func(values []T, index int) reflect.Value {
		return reflect.ValueOf(&values[index]).Elem()
	}

	var v T
	valueType := reflect.TypeOf(v)
	if valueType.Kind() == reflect.Pointer {
		valueType = valueType.Elem()
		valueOf = func(values []T, index int) reflect.Value {
			return reflect.ValueOf(values[index]).Elem()
		}
	}

	var encoders []encodeFunc
	for i, f := range reflect.VisibleFields(valueType) {
		if i != 0 {
			if _, err := io.WriteString(tw, "\t"); err != nil {
				return err
			}
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

		if _, err := io.WriteString(tw, name); err != nil {
			return err
		}
		encoders = append(encoders, encodeFuncOfStructField(f.Type, f.Index))
	}

	if _, err := io.WriteString(tw, "\n"); err != nil {
		return err
	}

	for n := range t.values {
		v := valueOf(t.values, n)
		w := io.Writer(tw)

		for i, enc := range encoders {
			if i != 0 {
				_, err := io.WriteString(w, "\t")
				if err != nil {
					return err
				}
			}
			if err := enc(w, v); err != nil {
				return err
			}
		}

		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}

	return tw.Flush()
}
