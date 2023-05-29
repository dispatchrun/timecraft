package imports

import (
	"container/list"
	"context"

	"github.com/stealthrocket/wazergo"
	"github.com/stealthrocket/wazergo/types"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
)

func Instantiate(ctx context.Context, runtime wazero.Runtime, fnCallStack func() api.CallStack, clones *list.List) (context.Context, error) {
	instance := wazergo.MustInstantiate(
		ctx,
		runtime,
		ForkHostModule,
		WithRuntime(runtime),
		WithFnCallStack(fnCallStack),
		WithClones(clones),
	)
	ctx = wazergo.WithModuleInstance(ctx, instance)
	return ctx, nil
}

const HostModuleName = "timecraft"

var ForkHostModule wazergo.HostModule[*Module] = functions{
	"fork": wazergo.F0((*Module).Fork),
}

type functions wazergo.Functions[*Module]

func (f functions) Name() string {
	return HostModuleName
}

func (f functions) Functions() wazergo.Functions[*Module] {
	return (wazergo.Functions[*Module])(f)
}

func (f functions) Instantiate(ctx context.Context, opts ...Option) (*Module, error) {
	mod := &Module{}
	wazergo.Configure(mod, opts...)
	return mod, nil
}

type Option = wazergo.Option[*Module]

type Module struct {
	runtime     wazero.Runtime
	forks       *list.List
	fnCallStack func() api.CallStack
}

type Forked struct {
	Module api.Module
	Stack  api.CallStack
}

func WithRuntime(runtime wazero.Runtime) Option {
	return wazergo.OptionFunc(func(m *Module) { m.runtime = runtime })
}

func WithClones(forks *list.List) Option {
	return wazergo.OptionFunc(func(m *Module) { m.forks = forks })
}

func WithFnCallStack(fn func() api.CallStack) Option {
	return wazergo.OptionFunc(func(m *Module) { m.fnCallStack = fn })
}

func (m *Module) Fork(ctx context.Context) types.Errno {
	callstack := m.fnCallStack()

	name := ctx.Value("module").(string)
	clone, err := m.runtime.CloneModule(ctx, name)
	if err != nil {
		return types.AsErrno(err)
	}
	f := Forked{
		Module: clone,
		Stack:  callstack,
	}
	if m.forks == nil {
		m.forks = list.New()
	}
	m.forks.PushBack(f)
	return 1
}

func (m *Module) Close(context.Context) error {
	//TODO
	m.forks = nil
	m.fnCallStack = nil
	return nil
}
