package debug

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/tetratelabs/wazero/sys"
)

const replUsage = `Commands:
  s, step     -- Step through events
  r, restart  -- Restart the debugger
  q, quit     -- Quit the debugger
  h, help     -- Show this usage information
`

var (
	// QuitError is raised (via panic) when the user calls q/quit.
	QuitError = errors.New("quitting from the debug REPL")

	// RestartError is raised (via panic) when the user calls r/restart.
	RestartError = errors.New("restarting from the debug REPL")
)

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
			r.printf("The module exited normally\n")
		case *sys.ExitError:
			r.printf("The module exited with code: %d\n", err.ExitCode())
		default:
			r.printf("The module exited with error: %v\n", err)
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
			r.printf("Quitting the debugger\n")
			r.closed = true
			panic(QuitError)
		case "r", "restart":
			r.printf("Restarting the debugger\n")
			r.closed = true
			panic(RestartError)
		case "h", "help" /* be forgiving: */, "?", "-h", "--help", `"help"`:
			r.print(replUsage)
			continue
		default:
			r.printf(`error: %q is not a valid command. See "help"\n`, command)
			continue
		}
	}
}

func (r *REPL) printf(s string, args ...any) (int, error) {
	return fmt.Fprintf(r.writer, s, args...)
}

func (r *REPL) print(args ...any) (int, error) {
	return fmt.Fprint(r.writer, args...)
}
