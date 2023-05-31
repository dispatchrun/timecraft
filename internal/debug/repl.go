package debug

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/tetratelabs/wazero/sys"
)

const replUsage = `Commands:
  s, step     -- Step through events
  q, quit     -- Quit the debugger
  h, help     -- Show this usage information
`

// REPL provides a read-eval-print loop for debugging WebAssembly modules.
type REPL struct {
	input  *bufio.Scanner
	tracer *Tracer
	writer io.Writer
	closed bool
}

// NewREPL creates a new REPL using the specified input and writer stream.
func NewREPL(input io.Reader, writer io.Writer) *REPL {
	return &REPL{
		input:  bufio.NewScanner(input),
		tracer: NewTracer(writer),
		writer: writer,
	}
}

func (r *REPL) OnEvent(ctx context.Context, event Event) {
	if r.closed {
		return
	}

	r.tracer.OnEvent(ctx, event)

	switch e := event.(type) {
	case *ModuleBeforeEvent:
		// noop
	case *ModuleAfterEvent:
		switch err := e.Error.(type) {
		case nil:
			r.printf("the module exited normally\n")
		case *sys.ExitError:
			r.printf("the module exited with code: %d\n", err.ExitCode())
		default:
			r.printf("the module exited with error: %v\n", err)
		}
	case *FunctionCallBeforeEvent:
		return
	case *FunctionCallAfterEvent:
		return
	case *FunctionCallAbortEvent:
		return
	case *SystemCallBeforeEvent:
		return
	case *SystemCallAfterEvent:
		return
	}

command_loop:
	for {
		r.print("> ")
		if !r.input.Scan() {
			r.closed = true
			return
		}

		input := strings.TrimSpace(r.input.Text())
		parts := strings.Split(input, " ")
		command := strings.TrimSpace(parts[0])

		switch command {
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
	return fmt.Fprintln(r.writer, args...)
}

func (r *REPL) printf(s string, args ...any) (int, error) {
	return fmt.Fprintf(r.writer, s, args...)
}

func (r *REPL) print(args ...any) (int, error) {
	return fmt.Fprint(r.writer, args...)
}
