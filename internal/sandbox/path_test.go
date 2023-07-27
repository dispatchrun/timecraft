package sandbox

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

func TestCleanPath(t *testing.T) {
	tests := []struct {
		input  string
		output string
	}{
		{"", ""},
		{".", "."},
		{"..", ".."},
		{"./", "."},
		{"/././././", "/"},
		{"hello/world", "hello/world"},
		{"/hello/world", "/hello/world"},
		{"/tmp/.././//test/", "/tmp/../test"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			path := cleanPath(test.input)
			assert.Equal(t, path, test.output)
		})
	}
}

func BenchmarkCleanPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = cleanPath("/tmp/.././//test/")
	}
}
