package debug

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

const replUsage = `Commands:
  s, step     -- Step through events
  c, continue -- Continue execution until a breakpoint is reached or the module exits
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
	input    *bufio.Scanner
	tracer   *Tracer
	writer   io.Writer
	stepping bool
	closed   bool
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

	// TODO: support breakpoints
	executing := true
	switch event.(type) {
	case *ModuleBeforeEvent, *ModuleAfterEvent:
		executing = false
	}
	if executing && !r.stepping {
		return
	}

read_input:
	r.print("> ")
	if !r.input.Scan() {
		r.closed = true
		panic(QuitError)
	}

	input := strings.TrimSpace(r.input.Text())
	parts := strings.Split(input, " ")
	command := strings.TrimSpace(parts[0])

	switch command {
	case "s", "step":
		r.stepping = true

	case "c", "continue":
		r.stepping = false

	case "r", "restart":
		r.print("Restarting...\n")
		r.closed = true
		panic(RestartError)

	case "q", "quit":
		r.closed = true
		panic(QuitError)

	case "h", "help" /* be forgiving: */, "?", "-h", "--help", `"help"`:
		r.print(replUsage)
		goto read_input

	case "":
		goto read_input

	default:
		r.printf(`error: %q is not a valid command. See "help"\n`, command)
		goto read_input
	}
}

func (r *REPL) printf(s string, args ...any) (int, error) {
	return fmt.Fprintf(r.writer, s, args...)
}

func (r *REPL) print(args ...any) (int, error) {
	return fmt.Fprint(r.writer, args...)
}
