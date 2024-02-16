package funccall

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

func Format(o io.Writer, reader stream.Reader[timemachine.Record]) error {
	tr := TraceReader{Records: reader}
	traces := make([]Trace, 1<<10)
	w := tabwriter.NewWriter(o, 0, 0, 1, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "duration\t symbol ")
	fmt.Fprintln(w, "---\t --- ")
	var s strings.Builder
	write := func(n int) {
		chunk := traces[:n]
		for x := range chunk {
			t := &chunk[x]
			fmt.Fprintf(w, "%v\t %s\n", t.Duration, formatSymbol(&s, t))
		}
		w.Flush()
	}

	for {
		n, err := tr.Read(traces)
		if err != nil {
			if errors.Is(err, io.EOF) {
				write(n)
				return nil
			}
			return err
		}
		write(n)
	}
}

func formatSymbol(s *strings.Builder, t *Trace) string {
	if t.Depth == 0 {
		return t.Name
	}
	s.Reset()
	s.Grow(t.Depth)
	for i := 0; i < t.Depth; i++ {
		s.WriteByte(' ')
	}
	s.WriteString(t.Name)
	return s.String()
}
