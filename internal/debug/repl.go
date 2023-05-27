package debug

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

// REPL provides a read-eval-print loop for debugging WebAssembly modules.
type REPL struct {
	input  *bufio.Scanner
	output io.Writer
	closed bool
}

// NewREPL creates a new REPL using the specified input and output stream.
func NewREPL(input io.Reader, output io.Writer) *REPL {
	return &REPL{
		input:  bufio.NewScanner(input),
		output: output,
	}
}

func (r *REPL) OnEvent(ctx context.Context, event Event) {
	if r.closed {
		return
	}
	switch e := event.(type) {
	case *FunctionCallBefore:
		r.println("FunctionCallBefore:", e.Function.DebugName())
	case *FunctionCallAfter:
		r.println("FunctionCallAfter:", e.Function.DebugName())
	case *WASICallBefore:
		r.println("WASICallBefore:", e.Syscall.ID().String())
	case *WASICallAfter:
		r.println("WASICallAfter:", e.Syscall.ID().String())
	}
	r.print("> ")
	if !r.input.Scan() {
		r.closed = true
		return
	}
	command := r.input.Text()
	r.println("You typed:", command)
}

func (r *REPL) println(args ...any) (int, error) {
	return fmt.Fprintln(r.output, args...)
}

func (r *REPL) printf(s string, args ...any) (int, error) {
	return fmt.Fprintf(r.output, s, args...)
}

func (r *REPL) print(args ...any) (int, error) {
	return fmt.Fprint(r.output, args...)
}
