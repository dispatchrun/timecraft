package human

import (
	"encoding"
	"flag"
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

func (p *Path) Set(s string) error {
	switch {
	case len(s) >= 2 && s[0] == '~' && s[1] == os.PathSeparator:
		home, ok := os.LookupEnv("HOME")
		if !ok {
			u, err := user.Current()
			if err != nil {
				return err
			}
			home = u.HomeDir
		}
		*p = Path(filepath.Join(home, s[2:]))
	default:
		*p = Path(s)
	}
	return nil
}

func (p *Path) UnmarshalText(b []byte) error {
	return p.Set(string(b))
}

var (
	_ encoding.TextUnmarshaler = (*Path)(nil)
	_ flag.Value               = (*Path)(nil)
)
