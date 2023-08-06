package fspath_test

import (
	"testing"

	"github.com/stealthrocket/timecraft/internal/assert"
	"github.com/stealthrocket/timecraft/internal/sandbox/fspath"
)

func TestDepth(t *testing.T) {
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
			assert.Equal(t, fspath.Depth(test.path), test.depth)
		})
	}
}

func TestJoin(t *testing.T) {
	tests := []struct {
		dir  string
		name string
		path string
	}{
		{"", "", ""},
		{".", ".", "./."},
		{".", "hello", "./hello"},
		{"hello", ".", "hello/."},
		{"/", "/", "/"},
		{"..//", ".", "../."},
		{"hello/world", "!", "hello/world/!"},
		{"/hello", "/world", "/hello/world"},
		{"/hello", "/world/", "/hello/world/"},
		{"//hello", "//world", "/hello/world"},
		{"//hello/", "//world//", "/hello/world/"},
		{"hello/../", "../world/./", "hello/../../world/"},
	}

	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			path := fspath.Join(test.dir, test.name)
			assert.Equal(t, path, test.path)
		})
	}
}

func TestClean(t *testing.T) {
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
		{"/tmp/.././//test/", "/tmp/../test/"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			path := fspath.Clean(test.input)
			assert.Equal(t, path, test.output)
		})
	}
}

func BenchmarkClean(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fspath.Clean("/tmp/.././//test/")
	}
}
