package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
	"github.com/stealthrocket/timecraft/internal/print/textprint"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"golang.org/x/exp/slices"
)

const describeUsage = `
Usage:	timecraft describe <resource type> <resource ids...> [options]

Options:
   -h, --help           Show this usage information
   -o, --ouptut format  Output format, one of: text, json, yaml
   -r, --registry path  Path to the timecraft registry (default to ~/.timecraft)
`

func describe(ctx context.Context, args []string) error {
	var (
		output       = outputFormat("text")
		registryPath = human.Path("~/.timecraft")
	)

	flagSet := newFlagSet("timecraft describe", describeUsage)
	customVar(flagSet, &output, "o", "output")
	customVar(flagSet, &registryPath, "r", "registry")
	parseFlags(flagSet, args)

	args = flagSet.Args()
	if len(args) == 0 {
		return errors.New(`expected one resource id as argument`)
	}
	resourceTypeLookup := args[0]
	resourceIDs := []string{}
	args = args[1:]

	for len(args) > 0 {
		parseFlags(flagSet, args)
		args = flagSet.Args()

		i := slices.IndexFunc(args, func(s string) bool {
			return strings.HasPrefix(s, "-")
		})
		if i < 0 {
			i = len(args)
		}
		resourceIDs = append(resourceIDs, args[:i]...)
		args = args[i:]
	}

	resource, ok := findResource(resourceTypeLookup, resources[:])
	if !ok {
		matchingResources := findMatchingResources(resourceTypeLookup, resources[:])
		if len(matchingResources) == 0 {
			return fmt.Errorf(`no resources matching '%s'`+useGet(), resourceTypeLookup)
		}
		return fmt.Errorf(`no resources matching '%s'

Did you mean?%s`, resourceTypeLookup, joinResourceTypes(matchingResources, "\n   "))
	}

	registry, err := openRegistry(registryPath)
	if err != nil {
		return err
	}

	if len(resourceIDs) == 0 {
		return fmt.Errorf(`no resources were specified, use 'timecraft describe %s <resources ids...>'`, resource.typ)
	}

	var lookup func(context.Context, *timemachine.Registry, string) (any, error)
	var writer stream.WriteCloser[any]
	switch output {
	case "json":
		lookup = resource.lookup
		writer = jsonprint.NewWriter[any](os.Stdout)
	case "yaml":
		lookup = resource.lookup
		writer = yamlprint.NewWriter[any](os.Stdout)
	default:
		lookup = resource.describe
		writer = textprint.NewWriter[any](os.Stdout)
	}
	defer writer.Close()

	readers := make([]stream.Reader[any], len(resourceIDs))
	for i, resource := range resourceIDs {
		readers[i] = &describeResourceReader{
			context:  ctx,
			registry: registry,
			resource: resource,
			lookup:   lookup,
		}
	}

	_, err = stream.Copy[any](writer, stream.MultiReader[any](readers...))
	return err
}

type describeResourceReader struct {
	context  context.Context
	registry *timemachine.Registry
	resource string
	lookup   func(context.Context, *timemachine.Registry, string) (any, error)
}

func (r *describeResourceReader) Read(values []any) (int, error) {
	if r.registry == nil {
		return 0, io.EOF
	}
	if len(values) == 0 {
		return 0, nil
	}
	defer func() { r.registry = nil }()
	v, err := r.lookup(r.context, r.registry, r.resource)
	if err != nil {
		return 0, err
	}
	values[0] = v
	return 1, io.EOF
}

func describeConfig(ctx context.Context, reg *timemachine.Registry, id string) (any, error) {
	d, err := reg.LookupDescriptor(ctx, format.ParseHash(id))
	if err != nil {
		return nil, err
	}
	c, err := reg.LookupConfig(ctx, d.Digest)
	if err != nil {
		return nil, err
	}
	var runtime string
	var version string
	r, err := reg.LookupRuntime(ctx, c.Runtime.Digest)
	if err != nil {
		runtime = "(unknown)"
		version = "(unknown)"
	} else {
		runtime = r.Runtime
		version = r.Version
	}
	desc := &configDescriptor{
		id: d.Digest.Short(),
		runtime: runtimeDescriptor{
			runtime: runtime,
			version: version,
		},
		modules: make([]moduleDescriptor, len(c.Modules)),
		args:    c.Args,
		env:     c.Env,
	}
	for i, module := range c.Modules {
		desc.modules[i] = moduleDescriptor{
			id:   module.Digest.Short(),
			name: moduleName(module),
			size: human.Bytes(module.Size),
		}
	}
	return desc, nil
}

func describeModule(ctx context.Context, reg *timemachine.Registry, id string) (any, error) {
	d, err := reg.LookupDescriptor(ctx, format.ParseHash(id))
	if err != nil {
		return nil, err
	}
	m, err := reg.LookupModule(ctx, d.Digest)
	if err != nil {
		return nil, err
	}
	desc := &moduleDescriptor{
		id:   d.Digest.Short(),
		name: moduleName(d),
		size: human.Bytes(len(m.Code)),
	}
	return desc, nil
}

func describeProcess(ctx context.Context, reg *timemachine.Registry, id string) (any, error) {
	processID, _, p, err := lookupProcessByLogID(ctx, reg, id)
	if err != nil {
		return nil, err
	}
	c, err := reg.LookupConfig(ctx, p.Config.Digest)
	if err != nil {
		return nil, err
	}

	var runtime string
	var version string
	r, err := reg.LookupRuntime(ctx, c.Runtime.Digest)
	if err != nil {
		runtime = "(unknown)"
		version = "(unknown)"
	} else {
		runtime = r.Runtime
		version = r.Version
	}

	desc := &processDescriptor{
		id:        processID,
		startTime: human.Time(p.StartTime.In(time.Local)),
		runtime: runtimeDescriptor{
			runtime: runtime,
			version: version,
		},
		modules: make([]moduleDescriptor, len(c.Modules)),
		args:    c.Args,
		env:     c.Env,
	}

	for i, module := range c.Modules {
		desc.modules[i] = moduleDescriptor{
			id:   module.Digest.Short(),
			name: moduleName(module),
			size: human.Bytes(module.Size),
		}
	}

	segments := reg.ListLogSegments(ctx, processID)
	defer segments.Close()

	i := stream.Iter[timemachine.LogSegment](segments)
	for i.Next() {
		v := i.Value()
		desc.log = append(desc.log, logSegment{
			number:    v.Number,
			size:      human.Bytes(v.Size),
			createdAt: human.Time(v.CreatedAt.In(time.Local)),
		})
	}
	if err := i.Err(); err != nil {
		return nil, err
	}

	return desc, nil
}

func describeRuntime(ctx context.Context, reg *timemachine.Registry, id string) (any, error) {
	d, err := reg.LookupDescriptor(ctx, format.ParseHash(id))
	if err != nil {
		return nil, err
	}
	r, err := reg.LookupRuntime(ctx, d.Digest)
	if err != nil {
		return nil, err
	}
	desc := &runtimeDescriptor{
		id:      d.Digest.Short(),
		runtime: r.Runtime,
		version: r.Version,
	}
	return desc, nil
}

func lookupConfig(ctx context.Context, reg *timemachine.Registry, id string) (any, error) {
	return lookup(ctx, reg, id, (*timemachine.Registry).LookupConfig)
}

func lookupModule(ctx context.Context, reg *timemachine.Registry, id string) (any, error) {
	return lookup(ctx, reg, id, (*timemachine.Registry).LookupModule)
}

func lookupProcess(ctx context.Context, reg *timemachine.Registry, id string) (any, error) {
	_, desc, proc, err := lookupProcessByLogID(ctx, reg, id)
	if err != nil {
		return nil, err
	}
	return descriptorAndData(desc, proc), nil
}

func lookupRuntime(ctx context.Context, reg *timemachine.Registry, id string) (any, error) {
	return lookup(ctx, reg, id, (*timemachine.Registry).LookupRuntime)
}

func lookup[T any](ctx context.Context, reg *timemachine.Registry, id string, lookup func(*timemachine.Registry, context.Context, format.Hash) (T, error)) (any, error) {
	desc, err := reg.LookupDescriptor(ctx, format.ParseHash(id))
	if err != nil {
		return nil, err
	}
	data, err := lookup(reg, ctx, desc.Digest)
	if err != nil {
		return nil, err
	}
	return descriptorAndData(desc, data), nil
}

func descriptorAndData(desc *format.Descriptor, data any) any {
	return &struct {
		Desc *format.Descriptor `json:"descriptor" yaml:"descriptor"`
		Data any                `json:"data"       yaml:"data"`
	}{
		Desc: desc,
		Data: data,
	}
}

func lookupProcessByLogID(ctx context.Context, reg *timemachine.Registry, id string) (format.UUID, *format.Descriptor, *format.Process, error) {
	processID, err := uuid.Parse(id)
	if err != nil {
		return processID, nil, nil, errors.New(`malformed process id (not a UUID)`)
	}
	manifest, err := reg.LookupLogManifest(ctx, processID)
	if err != nil {
		return processID, nil, nil, err
	}
	process, err := reg.LookupProcess(ctx, manifest.Process.Digest)
	if err != nil {
		return processID, nil, nil, err
	}
	return processID, manifest.Process, process, nil
}

type configDescriptor struct {
	id      string
	runtime runtimeDescriptor
	modules []moduleDescriptor
	args    []string
	env     []string
}

func (desc *configDescriptor) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "ID: %s\n", desc.id)
	fmt.Fprintf(w, "Runtime: %s (%s)\n", desc.runtime.runtime, desc.runtime.version)
	fmt.Fprintf(w, "Modules:\n")
	for _, module := range desc.modules {
		fmt.Fprintf(w, "  %s: %s (%v)\n", module.id, module.name, module.size)
	}
	fmt.Fprintf(w, "Args:\n")
	for _, arg := range desc.args {
		fmt.Fprintf(w, "  %s\n", arg)
	}
	fmt.Fprintf(w, "Env:\n")
	for _, env := range desc.env {
		fmt.Fprintf(w, "  %s\n", env)
	}
}

type moduleDescriptor struct {
	id   string
	name string
	size human.Bytes
}

func (desc *moduleDescriptor) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "ID: %s\n", desc.id)
	fmt.Fprintf(w, "Name: %s\n", desc.name)
	fmt.Fprintf(w, "Size: %v\n", desc.size)
}

type processDescriptor struct {
	id        format.UUID
	startTime human.Time
	runtime   runtimeDescriptor
	modules   []moduleDescriptor
	args      []string
	env       []string
	log       []logSegment
}

type logSegment struct {
	number    int
	size      human.Bytes
	createdAt human.Time
}

func (desc *processDescriptor) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "ID: %s\n", desc.id)
	fmt.Fprintf(w, "Start Time: %s, %s\n", desc.startTime, time.Time(desc.startTime).Format(time.RFC1123))
	fmt.Fprintf(w, "Runtime: %s (%s)\n", desc.runtime.runtime, desc.runtime.version)
	fmt.Fprintf(w, "Modules:\n")
	for _, module := range desc.modules {
		fmt.Fprintf(w, "  %s: %s (%v)\n", module.id, module.name, module.size)
	}
	fmt.Fprintf(w, "Args:\n")
	for _, arg := range desc.args {
		fmt.Fprintf(w, "  %s\n", arg)
	}
	fmt.Fprintf(w, "Env:\n")
	for _, env := range desc.env {
		fmt.Fprintf(w, "  %s\n", env)
	}
	fmt.Fprintf(w, "Log:\n")
	for _, log := range desc.log {
		fmt.Fprintf(w, "  segment %d: %s, created %s (%s)\n", log.number, log.size, log.createdAt, time.Time(log.createdAt).Format(time.RFC1123))
	}
}

type runtimeDescriptor struct {
	id      string
	runtime string
	version string
}

func (desc *runtimeDescriptor) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "ID: %s\n", desc.id)
	fmt.Fprintf(w, "Runtime: %s\n", desc.runtime)
	fmt.Fprintf(w, "Version: %s\n", desc.version)
}

func moduleName(module *format.Descriptor) string {
	name := module.Annotations["timecraft.module.name"]
	if name == "" {
		name = "(none)"
	}
	return name
}
