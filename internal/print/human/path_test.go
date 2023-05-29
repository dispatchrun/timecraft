package human

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	separator := string([]byte{filepath.Separator})

	tests := []struct {
		in  string
		out string
	}{
		{in: ".", out: "."},
		{in: separator, out: separator},
		{in: filepath.Join(".", "hello", "world"), out: filepath.Join(".", "hello", "world")},
		{in: filepath.Join("~", "hello", "world"), out: filepath.Join(os.Getenv("HOME"), "hello", "world")},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			path := Path("")

			if err := path.UnmarshalText([]byte(test.in)); err != nil {
				t.Error(err)
			}
			resolved, err := path.Resolve()
			if err != nil {
				t.Error(err)
			} else if resolved != test.out {
				t.Errorf("path mismatch: %q != %q", resolved, test.out)
			}
		})
	}
}
