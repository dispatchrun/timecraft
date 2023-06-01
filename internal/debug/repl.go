package debug

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

const replUsage = `
Debugger Commands:
  s, step             -- Step through events
  c, continue         -- Continue execution until a breakpoint is reached or the module exits
  t, trace <options>  -- Configure the tracer using a comma-separated list of options
  r, restart          -- Restart the debugger
  q, quit             -- Quit the debugger
  h, help             -- Show this usage information

Tracer Options:
  all        -- Enable all tracing
  functions  -- Enable function call tracing
  syscalls   -- Enable system call tracing
  stack      -- Enable function call stack params/results tracing
  timestamps -- Enable timestamps in tracer output
  relative   -- Show relative times between tracer lines
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
	case "t", "trace":
		if len(parts) == 1 {
			r.println(`error: expected a tracer option. See "help"`)
		}
		for _, opt := range strings.Split(parts[1], ",") {
			opt = strings.TrimSpace(opt)
			if len(opt) == 0 {
				continue
			}
			enable := true
			switch opt[0] {
			case '+':
				opt = opt[1:]
			case '-':
				enable = false
				opt = opt[1:]
			}
			switch opt {
			case "all":
				r.tracer.TraceFunctionCalls(enable)
				r.tracer.TraceSystemCalls(enable)
				r.tracer.TraceStack(enable)
				r.tracer.EnableTimestamps(enable)
				r.tracer.RelativeTimestamps(enable)
			case "functions":
				r.tracer.TraceFunctionCalls(enable)
			case "syscalls":
				r.tracer.TraceSystemCalls(enable)
			case "stack":
				r.tracer.TraceStack(enable)
			case "timestamps":
				r.tracer.EnableTimestamps(enable)
			case "relative":
				r.tracer.RelativeTimestamps(enable)
			}
		}
		goto read_input

	case "s", "step":
		if ctx.Err() != nil {
			r.println(`error: the module has exited. Try "restart", "quit" or "help"`)
			goto read_input
		}
		r.stepping = true

	case "c", "continue":
		if ctx.Err() != nil {
			r.println(`error: the module has exited. Try "restart", "quit" or "help"`)
			goto read_input
		}
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
		r.printf("error: %q is not a valid command. See \"help\"\n", command)
		goto read_input
	}
}

func (r *REPL) printf(s string, args ...any) (int, error) {
	return fmt.Fprintf(r.writer, s, args...)
}

func (r *REPL) print(args ...any) (int, error) {
	return fmt.Fprint(r.writer, args...)
}

func (r *REPL) println(args ...any) (int, error) {
	return fmt.Fprintln(r.writer, args...)
}
