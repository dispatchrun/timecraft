package textprint

import "io"

func QuoteBytes(w io.Writer) io.Writer {
	return NewPrefixWriter(Nolastline(w), []byte("    | "))
}
