package human

import (
	"encoding"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

// Path represents a path on the file system.
//
// The type interprets the special prefix "~/" as representing the home
// directory of the user that the program is running as.
type Path string

func (p Path) String() string {
	return string(p)
}

func (p Path) Get() any {
	return string(p)
}

func (p *Path) Set(s string) error {
	*p = Path(s)
	return nil
}

func (p *Path) UnmarshalText(b []byte) error {
	return p.Set(string(b))
}

func (p Path) Resolve() (string, error) {
	switch {
	case len(p) >= 2 && p[0] == '~' && p[1] == os.PathSeparator:
		home, ok := os.LookupEnv("HOME")
		if !ok {
			u, err := user.Current()
			if err != nil {
				return "", err
			}
			home = u.HomeDir
		}
		return filepath.Join(home, string(p[2:])), nil
	default:
		return string(p), nil
	}
}

var (
	_ fmt.Stringer             = Path("")
	_ encoding.TextUnmarshaler = (*Path)(nil)
	_ flag.Getter              = (*Path)(nil)
	_ flag.Value               = (*Path)(nil)
)
