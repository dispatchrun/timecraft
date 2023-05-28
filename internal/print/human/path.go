package human

import (
	"bytes"
	"os"
	"os/user"
	"path/filepath"
)

// Path represents a path on the file system.
//
// The type interprets the special prefix "~/" as representing the home
// directory of the user that the program is running as.
type Path string

func (p *Path) UnmarshalText(b []byte) error {
	switch {
	case bytes.HasPrefix(b, []byte{'~', filepath.Separator}):
		home, ok := os.LookupEnv("HOME")
		if !ok {
			u, err := user.Current()
			if err != nil {
				return err
			}
			home = u.HomeDir
		}
		*p = Path(filepath.Join(home, string(b[2:])))
	default:
		*p = Path(b)
	}
	return nil
}
