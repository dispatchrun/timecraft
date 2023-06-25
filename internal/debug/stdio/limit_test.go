package stdio_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stealthrocket/timecraft/internal/testing/assert"
	"github.com/stealthrocket/timecraft/internal/debug/stdio"
)

func TestLimit(t *testing.T) {
	const text = `hello world!
second line
third line
a
b
the last line has no return`

	for limit := 0; limit < 10; limit++ {
		t.Run("", func(t *testing.T) {
			lines := strings.Split(text, "\n")
			if len(lines) > limit {
				lines = lines[:limit]
				if limit > 0 {
					lines[limit-1] += "\n"
				}
			}
			output := strings.Join(lines, "\n")
			buffer := new(bytes.Buffer)
			_, err := buffer.ReadFrom(&stdio.Limit{
				R: strings.NewReader(text),
				N: limit,
			})
			assert.OK(t, err)
			assert.Equal(t, buffer.String(), output)
		})
	}
}
