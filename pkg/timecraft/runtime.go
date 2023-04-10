package timecraft

import (
	"context"
	"crypto/rand"
	"os"

	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

var runtimeConfig = wazero.NewRuntimeConfig().
	WithDebugInfoEnabled(true).
	WithCustomSections(true).
	WithCloseOnContextDone(true).
	WithCompilationCache(wazero.NewCompilationCache())

type Process struct {
	runtime   wazero.Runtime
	module    wazero.CompiledModule
	timecraft *wazergo.ModuleInstance[*Module]
}

func SpawnProcess(ctx context.Context, bytecode []byte) (*Process, error) {
	runtime := wazero.NewRuntimeWithConfig(ctx, runtimeConfig)
	wasi_snapshot_preview1.MustInstantiate(ctx, runtime)

	timecraft := wazergo.MustInstantiate(ctx, runtime, hostModule)

	module, err := runtime.CompileModule(ctx, bytecode)
	if err != nil {
		runtime.Close(ctx)
		return nil, err
	}
	return &Process{runtime, module, timecraft}, nil
}

func (p *Process) Close(ctx context.Context) error {
	return p.runtime.Close(ctx)
}

func (p *Process) Run(ctx context.Context) error {
	ctx = wazergo.WithModuleInstance(ctx, p.timecraft)
	// Instantiating the module invokes its _start function.
	m, err := p.runtime.InstantiateModule(ctx, p.module,
		// TODO: fix module config
		wazero.NewModuleConfig().
			WithStderr(os.Stderr).
			WithStdout(os.Stdout).
			WithSysWalltime().
			WithSysNanotime().
			WithSysNanosleep().
			WithRandSource(rand.Reader),
	)
	if err != nil {
		return err
	}
	return m.Close(ctx)
}
