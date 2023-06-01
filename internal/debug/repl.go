package debug

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/tetratelabs/wazero/api"
)

const replUsage = `
Debugger Commands:
  s, step             -- Step through events
  c, continue         -- Continue execution until a breakpoint is reached or the module exits
  b, break <options>  -- Configure a breakpoint using a comma-separated list of options
  t, trace <options>  -- Configure the tracer using a comma-separated list of options
  r, restart          -- Restart the debugger
  q, quit             -- Quit the debugger
  h, help             -- Show this usage information

Breakpoint Options:
  functions       -- Break at function calls
  syscalls        -- Break at system calls
  function=NAME   -- Break at function calls with a name that contains NAME
  syscall=NAME    -- Break at system calls with a name that contains NAME
  errno=ERRNO     -- Breat at system calls that return ERRNO

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
	input       *bufio.Scanner
	tracer      *Tracer
	writer      io.Writer
	breakpoints []breakpoint
	stepping    bool
	closed      bool
}

// NewREPL creates a new REPL using the specified input and writer stream.
func NewREPL(input io.Reader, writer io.Writer) *REPL {
	return &REPL{
		input:  bufio.NewScanner(input),
		tracer: NewTracer(writer),
		writer: writer,
	}
}

type breakpoint struct {
	functions    bool
	functionName string
	syscalls     bool
	syscallName  string
	errno        string
}

func (bp *breakpoint) matchFunction(fn api.FunctionDefinition) bool {
	if bp.functionName != "" {
		return strings.Contains(fn.DebugName(), bp.functionName)
	}
	return bp.functions
}

func (bp *breakpoint) matchSyscall(s wasicall.Syscall, after bool) bool {
	if bp.syscallName != "" {
		return strings.Contains(s.ID().String(), bp.syscallName)
	}
	if bp.errno != "" && after {
		return s.Error().Name() == bp.errno
	}
	return bp.syscalls
}

func (r *REPL) matchBreakpoint(event Event) bool {
	for _, bp := range r.breakpoints {
		switch e := event.(type) {
		case *FunctionCallBeforeEvent:
			if bp.matchFunction(e.Function) {
				return true
			}
		case *FunctionCallAfterEvent:
			if bp.matchFunction(e.Function) {
				return true
			}
		case *SystemCallBeforeEvent:
			if bp.matchSyscall(e.Syscall, false) {
				return true
			}
		case *SystemCallAfterEvent:
			if bp.matchSyscall(e.Syscall, true) {
				return true
			}
		}
	}
	return false
}

func (r *REPL) OnEvent(ctx context.Context, event Event) {
	if r.closed {
		return
	}

	r.tracer.OnEvent(ctx, event)

	executing := true
	switch event.(type) {
	case *ModuleBeforeEvent, *ModuleAfterEvent:
		executing = false
	}
	if executing {
		if !r.stepping && !r.matchBreakpoint(event) {
			return
		}
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

	case "b", "break", "breakpoint":
		if len(parts) == 1 {
			r.println(`error: expected a breakpoint option. See "help"`)
			goto read_input
		}
		var bp breakpoint
		for _, rawOpt := range strings.Split(parts[1], ",") {
			opt := strings.TrimSpace(rawOpt)
			if len(opt) == 0 {
				continue
			}
			var value string
			opt, value, _ = strings.Cut(opt, "=")
			opt = strings.TrimSpace(opt)
			value = strings.TrimSpace(value)
			switch {
			case opt == "functions":
				bp.functions = true
			case opt == "syscalls":
				bp.syscalls = true
			case opt == "function" && value != "":
				bp.functionName = value
			case opt == "syscall" && value != "":
				bp.syscallName = value
			case opt == "errno" && value != "":
				bp.errno = strings.ToUpper(value)
			default:
				r.printf("error: invalid breakpoint option %q. See \"help\"", rawOpt)
				goto read_input
			}
		}
		r.breakpoints = append(r.breakpoints, bp)
		goto read_input

	case "t", "trace", "tracer":
		if len(parts) == 1 {
			r.println(`error: expected a tracer option. See "help"`)
			goto read_input
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
			default:
				r.printf("error: invalid tracer option %q. See \"help\"", opt)
				goto read_input
			}
		}
		goto read_input

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
