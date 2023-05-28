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
		out Path
	}{
		{in: ".", out: "."},
		{in: separator, out: Path(separator)},
		{in: filepath.Join(".", "hello", "world"), out: Path(filepath.Join(".", "hello", "world"))},
		{in: filepath.Join("~", "hello", "world"), out: Path(filepath.Join(os.Getenv("HOME"), "hello", "world"))},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			path := Path("")

			if err := path.UnmarshalText([]byte(test.in)); err != nil {
				t.Error(err)
			} else if path != test.out {
				t.Errorf("path mismatch: %q != %q", path, test.out)
			}
		})
	}
}
