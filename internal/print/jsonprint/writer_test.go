package jsonprint_test

import (
	"bytes"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
)

type tag struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

func TestWriteNothing(t *testing.T) {
	b := new(bytes.Buffer)
	w := jsonprint.NewWriter[tag](b)
	assert.OK(t, w.Close())
	assert.Equal(t, b.String(), "")
}

func TestWriteValues(t *testing.T) {
	b := new(bytes.Buffer)
	w := jsonprint.NewWriter[tag](b)
	_, err := w.Write([]tag{
		{Name: "one", Value: "1"},
		{Name: "two", Value: "2"},
		{Name: "three", Value: "3"},
	})
	assert.OK(t, err)
	assert.OK(t, w.Close())
	assert.Equal(t, b.String(), `{
  "name": "one",
  "value": "1"
}
{
  "name": "two",
  "value": "2"
}
{
  "name": "three",
  "value": "3"
}
`)
}
