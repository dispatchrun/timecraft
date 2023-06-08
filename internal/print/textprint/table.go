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

func Header[T any](enable bool) TableOption[T] {
	return func(t *tableWriter[T]) { t.header = enable }
}

func List[T any](enable bool) TableOption[T] {
	return func(t *tableWriter[T]) { t.list = enable }
}

func OrderBy[T any](f func(T, T) bool) TableOption[T] {
	return func(t *tableWriter[T]) { t.orderBy = f }
}

func NewTableWriter[T any](w io.Writer, opts ...TableOption[T]) stream.WriteCloser[T] {
	t := &tableWriter[T]{
		output: w,
		header: true,
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

type tableWriter[T any] struct {
	output  io.Writer
	values  []T
	header  bool
	list    bool
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

	var columns []string
	var encoders []encodeFunc
	for _, f := range reflect.VisibleFields(valueType) {
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
		columns = append(columns, name)
		encoders = append(encoders, encodeFuncOfStructField(f.Type, f.Index))
	}

	if t.list {
		columns = columns[:1]
		encoders = encoders[:1]
	}

	if t.header {
		for _, name := range columns {
			if _, err := io.WriteString(tw, name); err != nil {
				return err
			}
			if _, err := io.WriteString(tw, "\t"); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(tw, "\n"); err != nil {
			return err
		}
	}

	for n := range t.values {
		v := valueOf(t.values, n)
		w := io.Writer(tw)

		for _, enc := range encoders {
			if err := enc(w, v); err != nil {
				return err
			}
			_, err := io.WriteString(w, "\t")
			if err != nil {
				return err
			}
		}

		if _, err := io.WriteString(w, "\n"); err != nil {
			return err
		}
	}

	return tw.Flush()
}
