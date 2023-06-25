package yamlprint_test

import (
	"bytes"
	"testing"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
)

type tag struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

func TestWriteNothing(t *testing.T) {
	b := new(bytes.Buffer)
	w := yamlprint.NewWriter[tag](b)
	assert.OK(t, w.Close())
	assert.Equal(t, b.String(), "")
}

func TestWriteValues(t *testing.T) {
	b := new(bytes.Buffer)
	w := yamlprint.NewWriter[tag](b)
	_, err := w.Write([]tag{
		{Name: "one", Value: "1"},
		{Name: "two", Value: "2"},
		{Name: "three", Value: "3"},
	})
	assert.OK(t, err)
	assert.OK(t, w.Close())
	assert.Equal(t, b.String(), `name: one
value: "1"
---
name: two
value: "2"
---
name: three
value: "3"
`)
}
