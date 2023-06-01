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

	enable                    bool
	enableIndent              bool
	enableFunctionCallTracing bool
	enableStackParamTracing   bool
	enableStackResultTracing  bool
	enableSystemCallTracing   bool

	indent int
}

// NewTracer creates a new Tracer.
func NewTracer(writer io.Writer) *Tracer {
	return &Tracer{
		writer:                    writer,
		enable:                    true,
		enableIndent:              true,
		enableSystemCallTracing:   true,
		enableFunctionCallTracing: true,
		enableStackParamTracing:   true,
		enableStackResultTracing:  true,
	}
}

func (t *Tracer) EnableIndent(enable bool) {
	t.enableIndent = enable
}

func (t *Tracer) EnableFunctionCallTracing(enable bool) {
	t.enableFunctionCallTracing = enable
}

func (t *Tracer) EnableSystemCallTracing(enable bool) {
	t.enableFunctionCallTracing = enable
}

func (t *Tracer) EnableStackParamTracing(enable bool) {
	t.enableStackParamTracing = enable
}

func (t *Tracer) EnableStackResultTracing(enable bool) {
	t.enableStackResultTracing = enable
}

func (t *Tracer) OnEvent(ctx context.Context, event Event) {
	switch e := event.(type) {
	case *ModuleBeforeEvent:
		t.ModuleBefore(ctx, e.Module)
	case *ModuleAfterEvent:
		t.ModuleAfter(ctx, e.Error)
	case *FunctionCallBeforeEvent:
		t.FunctionCallBefore(ctx, e.Module, e.Function, e.Params)
	case *FunctionCallAfterEvent:
		t.FunctionCallAfter(ctx, e.Module, e.Function, e.Results)
	case *FunctionCallAbortEvent:
		t.FunctionCallAbort(ctx, e.Module, e.Function, e.Error)
	case *SystemCallBeforeEvent:
		t.SystemCallBefore(ctx, e.Syscall)
	case *SystemCallAfterEvent:
		t.SystemCallAfter(ctx, e.Syscall)
	}
}

func (t *Tracer) ModuleBefore(ctx context.Context, mod wazero.CompiledModule) {

}

func (t *Tracer) ModuleAfter(ctx context.Context, err any) {
	switch e := err.(type) {
	case nil:
		t.print("The module exited normally\n")
	case *sys.ExitError:
		t.printf("The module exited with code: %d\n", e.ExitCode())
	default:
		t.printf("The module exited with error: %v\n", e)
	}
}

func (t *Tracer) FunctionCallBefore(ctx context.Context, mod api.Module, fn api.FunctionDefinition, params []uint64) {
	if !t.enable || !t.enableFunctionCallTracing {
		return
	}

	t.printLine(func() {
		t.print(fn.DebugName())
		if t.enableStackParamTracing {
			t.print(color.HiBlackString(" <= "))
			t.printStack(params)
		}
	})

	t.indent++
}

func (t *Tracer) FunctionCallAfter(ctx context.Context, mod api.Module, fn api.FunctionDefinition, results []uint64) {
	if !t.enable || !t.enableFunctionCallTracing {
		return
	}

	t.indent--

	if !t.enableStackResultTracing {
		return
	}

	t.printLine(func() {
		t.print(fn.DebugName())
		t.print(color.HiBlackString(" => "))
		t.printStack(results)
	})
}

func (t *Tracer) FunctionCallAbort(ctx context.Context, mod api.Module, fn api.FunctionDefinition, err error) {
	if !t.enable || !t.enableFunctionCallTracing {
		return
	}

	t.indent--

	t.printLine(func() {
		t.print(fn.DebugName(), "")
		t.print(color.HiBlackString(" => "))
		t.print("(")
		if _, ok := err.(*sys.ExitError); ok {
			t.print(color.YellowString("exiting"))
		} else {
			t.print(color.RedString(err.Error()))
		}
		t.print(")")
	})
}

func (t *Tracer) SystemCallBefore(ctx context.Context, s wasicall.Syscall) {
	if !t.enable || !t.enableSystemCallTracing {
		return
	}

	t.printLine(func() {
		t.print(color.MagentaString(s.ID().String()))

		t.print(color.HiBlackString(" <= "))

		t.print("(")
		for i, param := range s.Params() {
			if i > 0 {
				t.print(", ")
			}
			t.printf("%v", param)
		}
		t.print(")")
	})
}

func (t *Tracer) SystemCallAfter(ctx context.Context, s wasicall.Syscall) {
	if !t.enable || !t.enableSystemCallTracing {
		return
	}

	t.printLine(func() {
		t.print(strings.Repeat(" ", len(s.ID().String())))

		t.print(color.HiBlackString(" => "))

		t.print("(")
		results := s.Results()
		errno := s.Error()
		for i, result := range results {
			if i > 0 {
				t.print(", ")
			}
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
		}
		t.print(")")
	})
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
	fmt.Fprintf(t.writer, color.BlackString("%d "), time.Now().UnixNano())

	if t.enableIndent && t.indent > 0 {
		return fmt.Fprint(t.writer, strings.Repeat(" ", t.indent))
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
