package imports_test

import (
	"bytes"
	"container/list"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stealthrocket/timecraft/internal/imports"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

// TODO: figure out how to get the call engine from the start function.
func TestFork(t *testing.T) {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	bin := "../../../testdata/fork/fork_go.wasm"
	fh, err := os.Open(bin)
	if err != nil {
		t.Fatal(err)
	}

	code, err := io.ReadAll(fh)
	if err != nil {
		t.Fatal(err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	clones := list.New()

	var fn api.Function
	getCallStack := func() api.CallStack {
		return fn.CallStack()
	}

	ctx, err = imports.Instantiate(ctx, runtime, getCallStack, clones)
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := runtime.CompileModule(ctx, code)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("spawnWithWasi", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w
		stdoutC := make(chan []byte)
		go func() {
			var buf bytes.Buffer
			io.Copy(&buf, r)
			stdoutC <- buf.Bytes()
		}()
		defer func() {
			os.Stdout = old
		}()

		mod, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithStdout(os.Stdout).WithName("main"))
		if err != nil {
			t.Fatal(err)
		}
		defer mod.Close(ctx)

		fnName := "spawnWithWasi"

		fn = mod.ExportedFunction(fnName)
		_, err = fn.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}

		e := clones.Front()
		if e == nil {
			t.Fatal("a clone was expected")
		}

		forked := e.Value.(imports.Forked)

		child := forked.Module
		defer child.Close(ctx)
		childFn := child.ResumeFunction(child, mod, fnName, forked.Stack)
		_, err = childFn.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}

		w.Close()
		out := <-stdoutC
		if string(out) != "child\n" {
			t.Error("unexpected stdout from child")
			t.Logf("got: %s", string(out))
		}
	})

	t.Run("forkWithPrintln", func(t *testing.T) {
		fnName := "forkWithPrintln2"

		mod, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithStdout(os.Stdout).WithName("main"))
		if err != nil {
			t.Fatal(err)
		}

		fn = mod.ExportedFunction(fnName)
		println("\n==== call parent")
		_, err = fn.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}

		e := clones.Front()
		if e == nil {
			t.Fatal("a clone was expected")
		}

		forked := e.Value.(imports.Forked)

		child := forked.Module
		childFn := child.ResumeFunction(child, mod, fnName, forked.Stack)
		println("\n==== call child")
		_, err = childFn.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestForkRust(t *testing.T) {
	ctx := context.Background()
	runtime := wazero.NewRuntime(ctx)

	bin := "../../../testdata/fork/fork_rs.wasm"
	fh, err := os.Open(bin)
	if err != nil {
		t.Fatal(err)
	}

	code, err := io.ReadAll(fh)
	if err != nil {
		t.Fatal(err)
	}

	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	clones := list.New()

	var fn api.Function
	getCallStack := func() api.CallStack {
		return fn.CallStack()
	}
	ctx, err = imports.Instantiate(ctx, runtime, getCallStack, clones)
	if err != nil {
		t.Fatal(err)
	}

	compiled, err := runtime.CompileModule(ctx, code)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("forkExit", func(t *testing.T) {
		fnName := "forkExit"

		mod, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithStdout(os.Stdout).WithName("main"))
		if err != nil {
			t.Fatal(err)
		}
		defer mod.Close(ctx)

		fn = mod.ExportedFunction(fnName)
		_, err = fn.Call(ctx)
		if err != nil {
			if exitErr, ok := err.(*sys.ExitError); ok {
				if exitErr.ExitCode() != 2 {
					t.Fatal("wrong exit code for parent")
				}
			} else {
				t.Fatal(err)
			}
		}

		e := clones.Front()
		if e == nil {
			t.Fatal("a clone was expected")
		}
		clones.Remove(e)

		forked := e.Value.(imports.Forked)

		child := forked.Module
		defer child.Close(ctx)
		childFn := child.ResumeFunction(child, mod, fnName, forked.Stack)
		_, err = childFn.Call(ctx)
		if err != nil {
			if exitErr, ok := err.(*sys.ExitError); ok {
				if exitErr.ExitCode() != 42 {
					t.Fatal("wrong exit code for parent")
				}
			} else {
				t.Fatal(err)
			}
		}
	})

	t.Run("forkWithWasi", func(t *testing.T) {
		fnName := "forkWithWasi"

		mod, err := runtime.InstantiateModule(ctx, compiled, wazero.NewModuleConfig().WithStdout(os.Stdout).WithName("main"))
		if err != nil {
			t.Fatal(err)
		}

		fn = mod.ExportedFunction(fnName)
		_, err = fn.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}

		e := clones.Front()
		if e == nil {
			t.Fatal("a clone was expected")
		}
		clones.Remove(e)

		forked := e.Value.(imports.Forked)

		child := forked.Module
		childFn := child.ResumeFunction(child, mod, fnName, forked.Stack)
		_, err = childFn.Call(ctx)
		if err != nil {
			t.Fatal(err)
		}
	})
}
