package textprint_test

import (
	"bytes"
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/print/textprint"
)

type person struct {
	FirstName string `text:"FIRST NAME"`
	LastName  string `text:"LAST NAME"`
	Age       int    `text:"AGE"`
}

func TestTableWriteNothing(t *testing.T) {
	b := new(bytes.Buffer)
	w := textprint.NewTableWriter[person](b)
	assert.OK(t, w.Close())
	assert.Equal(t, b.String(), "FIRST NAME  LAST NAME  AGE\n")
}

func TestTableWriteValues(t *testing.T) {
	b := new(bytes.Buffer)
	w := textprint.NewTableWriter[person](b)
	_, err := w.Write([]person{
		{FirstName: "Luke", LastName: "Skywalker", Age: 19},
		{FirstName: "Leia", LastName: "Skywalker", Age: 19},
		{FirstName: "Han", LastName: "Solo", Age: 19},
	})
	assert.OK(t, err)
	assert.OK(t, w.Close())
	assert.Equal(t, b.String(), `FIRST NAME  LAST NAME  AGE
Luke        Skywalker  19
Leia        Skywalker  19
Han         Solo       19
`)
}
