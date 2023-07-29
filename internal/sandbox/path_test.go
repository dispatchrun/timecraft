package sandbox

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
)

func TestFilePathDepth(t *testing.T) {
	tests := []struct {
		path  string
		depth int
	}{
		{"", 0},
		{".", 0},
		{"/", 0},
		{"..", 0},
		{"/..", 0},
		{"a/b/c", 3},
		{"//hello//world/", 2},
		{"/../path/././to///file/..", 2},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			assert.Equal(t, filePathDepth(test.path), test.depth)
		})
	}
}

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
