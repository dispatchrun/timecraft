package debug

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/tetratelabs/wazero/sys"
)

const replUsage = `Debugger commands:
  s, step   -- Step through events
  q, quit   -- Quit the debugger
  h, help   -- Show this usage information
`

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
	case *ModuleBefore:
		r.println("ModuleBefore")
	case *ModuleAfter:
		r.println("ModuleAfter")
	case *FunctionCallBefore:
		r.println("FunctionCallBefore:", e.Function.DebugName())
	case *FunctionCallAfter:
		r.println("FunctionCallAfter:", e.Function.DebugName())
	case *FunctionCallAbort:
		r.println("FunctionCallAbort:", e.Function.DebugName(), e.Error)
	case *WASICallBefore:
		r.println("WASICallBefore:", e.Syscall.ID().String())
	case *WASICallAfter:
		r.println("WASICallAfter:", e.Syscall.ID().String())
	}

command_loop:
	for {
		r.print("> ")
		if !r.input.Scan() {
			r.closed = true
			return
		}
		switch command := strings.TrimSpace(r.input.Text()); command {
		case "":
			continue
		case "s", "step":
			break command_loop
		case "q", "quit":
			r.closed = true
			panic(sys.NewExitError(0))
		case "h", "help" /* be forgiving: */, "?", "-h", "--help":
			r.print(replUsage)
			continue
		default:
			r.printf("error: %q is not a valid command\n", command)
			continue
		}
	}
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
