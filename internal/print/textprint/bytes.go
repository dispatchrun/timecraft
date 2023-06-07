package textprint

import "io"

func QuoteBytes(w io.Writer) io.Writer {
	return Prefixlines(Nolastline(w), []byte("    | "))
}
