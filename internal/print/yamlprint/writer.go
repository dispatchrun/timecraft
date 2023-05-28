package yamlprint

import (
	"io"

	"gopkg.in/yaml.v3"

	"github.com/stealthrocket/timecraft/internal/stream"
)

func NewWriter[T any](w io.Writer) stream.WriteCloser[T] {
	e := yaml.NewEncoder(w)
	e.SetIndent(2)
	return writer[T]{e}
}

type writer[T any] struct{ *yaml.Encoder }

func (w writer[T]) Write(values []T) (int, error) {
	for i := range values {
		if err := w.Encode(values[i]); err != nil {
			return i, err
		}
	}
	return len(values), nil
}

func (w writer[T]) Close() error {
	err := w.Encoder.Close()
	if err != nil {
		if s := err.Error(); s == `yaml: expected STREAM-START` {
			err = nil
		}
	}
	return err
}
