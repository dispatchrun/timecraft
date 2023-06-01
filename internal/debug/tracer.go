package debug

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/sys"
)

// Tracer traces a WebAssembly module's function calls and WASI system
// calls.
type Tracer struct {
	writer io.Writer

	traceFunctionCalls bool
	traceStack         bool
	traceSystemCalls   bool
	enableTimestamps   bool
	relativeTimestamps bool

	previousTime time.Time
	depth        int
}

// NewTracer creates a new Tracer.
func NewTracer(writer io.Writer) *Tracer {
	return &Tracer{writer: writer}
}

func (t *Tracer) TraceFunctionCalls(enable bool) {
	t.traceFunctionCalls = enable
}

func (t *Tracer) TraceSystemCalls(enable bool) {
	t.traceSystemCalls = enable
}

func (t *Tracer) TraceStack(enable bool) {
	t.traceStack = enable
}

func (t *Tracer) EnableTimestamps(enable bool) {
	t.enableTimestamps = enable
}

func (t *Tracer) RelativeTimestamps(enable bool) {
	t.relativeTimestamps = enable
}

func (t *Tracer) OnEvent(ctx context.Context, event Event) {
	switch e := event.(type) {
	case *ModuleBeforeEvent:
		t.moduleBefore(ctx, e.Module)
	case *ModuleAfterEvent:
		t.moduleAfter(ctx, e.Error)
	case *FunctionCallBeforeEvent:
		t.functionCallBefore(ctx, e.Module, e.Function, e.Params)
	case *FunctionCallAfterEvent:
		t.functionCallAfter(ctx, e.Module, e.Function, e.Results)
	case *FunctionCallAbortEvent:
		t.functionCallAbort(ctx, e.Module, e.Function, e.Error)
	case *SystemCallBeforeEvent:
		t.systemCallBefore(ctx, e.Syscall)
	case *SystemCallAfterEvent:
		t.systemCallAfter(ctx, e.Syscall)
	}

	t.previousTime = time.Now()
}

func (t *Tracer) moduleBefore(ctx context.Context, mod wazero.CompiledModule) {
}

func (t *Tracer) moduleAfter(ctx context.Context, err any) {
	switch e := err.(type) {
	case nil:
		t.print("The module exited normally\n")
	case *sys.ExitError:
		t.printf("The module exited with code: %d\n", e.ExitCode())
	default:
		t.printf("The module exited with error: %v\n", e)
	}
}

func (t *Tracer) functionCallBefore(ctx context.Context, mod api.Module, fn api.FunctionDefinition, params []uint64) {
	if !t.traceFunctionCalls {
		return
	}

	t.printLine(func() {
		t.print(color.BlackString("→ "))
		t.print(fn.DebugName())
		if t.traceStack {
			t.printStack(params)
		}
	})

	t.depth++
}

func (t *Tracer) functionCallAfter(ctx context.Context, mod api.Module, fn api.FunctionDefinition, results []uint64) {
	if !t.traceFunctionCalls {
		return
	}

	t.depth--

	t.printLine(func() {
		t.print(color.BlackString("← "))
		t.print(fn.DebugName())
		if t.traceStack {
			t.print(color.HiBlackString(" => "))
			t.printStack(results)
		}
	})
}

func (t *Tracer) functionCallAbort(ctx context.Context, mod api.Module, fn api.FunctionDefinition, err error) {
	if !t.traceFunctionCalls {
		return
	}

	t.depth--

	t.printLine(func() {
		t.print(color.BlackString("← "))
		t.print(fn.DebugName())
		t.print(" (")
		if _, ok := err.(*sys.ExitError); ok {
			t.print(color.YellowString("exiting"))
		} else {
			t.print(color.RedString(err.Error()))
		}
		t.print(")")
	})
}

func (t *Tracer) systemCallBefore(ctx context.Context, s wasicall.Syscall) {
	if !t.traceSystemCalls {
		return
	}

	t.printLine(func() {
		t.print(color.MagentaString(s.ID().String()))
	})
	for _, param := range s.Params() {
		t.printLine(func() {
			t.print(color.HiBlackString("   <= "))
			t.printf("%v", param)
		})
	}
}

func (t *Tracer) systemCallAfter(ctx context.Context, s wasicall.Syscall) {
	if !t.traceSystemCalls {
		return
	}
	results := s.Results()
	errno := s.Error()
	for i, result := range results {
		t.printLine(func() {
			t.print(color.HiBlackString("   => "))
			if i == len(results)-1 && errno == result {
				switch errno {
				case wasi.ESUCCESS:
					t.print(color.GreenString("OK"))
				case wasi.EAGAIN:
					t.print(color.YellowString("EAGAIN"))
				default:
					t.print(color.HiRedString(errno.Name()))
				}
			} else {
				t.printf("%v", result)
			}
		})
	}
}

func (t *Tracer) printStack(values []uint64) {
	t.print("(")
	for i, v := range values {
		if i > 0 {
			t.print(", ")
		}
		t.printf("%#x", v)
	}
	t.print(")")
}

func (t *Tracer) printLine(fn func()) {
	_, _ = t.printPrefix()
	defer func() { _, _ = t.printSuffix() }()

	fn()
}

func (t *Tracer) printPrefix() (int, error) {
	if t.enableTimestamps {
		switch {
		case t.relativeTimestamps && t.previousTime == time.Time{}:
			fmt.Fprint(t.writer, "")
		case t.relativeTimestamps:
			elapsed := time.Since(t.previousTime)
			switch {
			case elapsed < time.Microsecond:
				fmt.Fprintf(t.writer, color.HiBlackString("%+ 4dns"), elapsed)
			case elapsed < time.Millisecond:
				fmt.Fprintf(t.writer, color.HiBlackString("%+ 4dµs"), elapsed/time.Microsecond)
			case elapsed < time.Second:
				fmt.Fprintf(t.writer, color.YellowString("%+ 4dms"), elapsed/time.Millisecond)
			case elapsed < 1000*time.Second:
				fmt.Fprintf(t.writer, color.RedString("%+ 4ds "), elapsed/time.Second)
			default:
				panic("not implemented")
			}
		default:
			fmt.Fprintf(t.writer, color.HiBlackString("%d "), time.Now().UnixNano())
		}
	}

	if t.traceFunctionCalls && t.depth > 0 {
		return fmt.Fprint(t.writer, strings.Repeat(" ", t.depth))
	}

	return 0, nil
}

func (t *Tracer) printSuffix() (int, error) {
	return t.writer.Write([]byte("\n"))
}

func (t *Tracer) printf(s string, args ...any) (int, error) {
	return fmt.Fprintf(t.writer, s, args...)
}

func (t *Tracer) print(args ...any) (int, error) {
	return fmt.Fprint(t.writer, args...)
}
