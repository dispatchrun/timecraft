package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
	"github.com/stealthrocket/timecraft/internal/print/textprint"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
)

const getUsage = `
Usage:	timecraft get <resource type> [options]

   The get sub-command gives access to the state of the time machine registry.
   The command must be followed by the name of resources to display, which must
   be one of config, module, process, or runtime.
   (the command also accepts plurals and abbreviations of the resource names)

Examples:

   $ timecraft get modules
   MODULE ID     MODULE NAME  SIZE
   9d7b7563baf3  app.wasm     6.82 MiB

   $ timecraft get modules -o json
   {
     "mediaType": "application/vnd.timecraft.module.v1+wasm",
     "digest": "sha256:9d7b7563baf3702cf24ed3688dc9a58faef2d0ac586041cb2dc95df919f5e5f2",
     "size": 7150231,
     "annotations": {
       "timecraft.module.name": "app.wasm",
       "timecraft.object.created-at": "2023-05-28T21:52:26Z"
     }
   }

Options:
   -h, --help           Show this usage information
   -o, --ouptut format  Output format, one of: text, json, yaml
   -r, --registry path  Path to the timecraft registry (default to ~/.timecraft)
`

type resource struct {
	typ       string
	alt       []string
	mediaType format.MediaType
	get       func(context.Context, io.Writer, *timemachine.Registry) stream.WriteCloser[*format.Descriptor]
	describe  func(context.Context, *timemachine.Registry, string) (any, error)
	lookup    func(context.Context, *timemachine.Registry, string) (any, error)
}

var resources = [...]resource{
	{
		typ:       "config",
		alt:       []string{"conf", "configs"},
		mediaType: format.TypeTimecraftConfig,
		get:       getConfigs,
		describe:  describeConfig,
		lookup:    lookupConfig,
	},
	{
		typ:       "module",
		alt:       []string{"mo", "mod", "mods", "modules"},
		mediaType: format.TypeTimecraftModule,
		get:       getModules,
		describe:  describeModule,
		lookup:    lookupModule,
	},
	{
		typ:       "process",
		alt:       []string{"ps", "proc", "procs", "processes"},
		mediaType: format.TypeTimecraftProcess,
		get:       getProcesses,
		describe:  describeProcess,
		lookup:    lookupProcess,
	},
	{
		typ:       "runtime",
		alt:       []string{"rt", "runtimes"},
		mediaType: format.TypeTimecraftRuntime,
		get:       getRuntimes,
		describe:  describeRuntime,
		lookup:    lookupRuntime,
	},
}

func get(ctx context.Context, args []string) error {
	var (
		timeRange    = timemachine.Since(time.Unix(0, 0))
		output       = outputFormat("text")
		registryPath = human.Path("~/.timecraft")
	)

	flagSet := newFlagSet("timecraft get", getUsage)
	customVar(flagSet, &output, "o", "output")
	customVar(flagSet, &registryPath, "r", "registry")
	parseFlags(flagSet, args)

	args = flagSet.Args()
	if len(args) == 0 {
		return errors.New(`expected exactly one resource type as argument` + useGet())
	}
	resourceTypeLookup := args[0]
	parseFlags(flagSet, args[1:])

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

	reader := registry.ListResources(ctx, resource.mediaType, timeRange)
	defer reader.Close()

	var writer stream.WriteCloser[*format.Descriptor]
	switch output {
	case "json":
		writer = jsonprint.NewWriter[*format.Descriptor](os.Stdout)
	case "yaml":
		writer = yamlprint.NewWriter[*format.Descriptor](os.Stdout)
	default:
		writer = resource.get(ctx, os.Stdout, registry)
	}
	defer writer.Close()

	_, err = stream.Copy[*format.Descriptor](writer, reader)
	return err
}

func getConfigs(ctx context.Context, w io.Writer, r *timemachine.Registry) stream.WriteCloser[*format.Descriptor] {
	type config struct {
		ID      string      `text:"CONFIG ID"`
		Runtime string      `text:"RUNTIME"`
		Modules int         `text:"MODULES"`
		Size    human.Bytes `text:"SIZE"`
	}
	return newDescTableWriter(w, func(desc *format.Descriptor) (config, error) {
		c, err := r.LookupConfig(ctx, desc.Digest)
		if err != nil {
			return config{}, err
		}
		r, err := r.LookupRuntime(ctx, c.Runtime.Digest)
		if err != nil {
			return config{}, err
		}
		return config{
			ID:      desc.Digest.Short(),
			Runtime: r.Runtime + " (" + r.Version + ")",
			Modules: len(c.Modules),
			Size:    human.Bytes(desc.Size),
		}, nil
	})
}

func getModules(ctx context.Context, w io.Writer, r *timemachine.Registry) stream.WriteCloser[*format.Descriptor] {
	type module struct {
		ID   string      `text:"MODULE ID"`
		Name string      `text:"MODULE NAME"`
		Size human.Bytes `text:"SIZE"`
	}
	return newDescTableWriter(w, func(desc *format.Descriptor) (module, error) {
		name := desc.Annotations["timecraft.module.name"]
		if name == "" {
			name = "(none)"
		}
		return module{
			ID:   desc.Digest.Short(),
			Name: name,
			Size: human.Bytes(desc.Size),
		}, nil
	})
}

func getProcesses(ctx context.Context, w io.Writer, r *timemachine.Registry) stream.WriteCloser[*format.Descriptor] {
	type process struct {
		ID        format.UUID `text:"PROCESS ID"`
		StartTime human.Time  `text:"STARTED"`
	}
	return newDescTableWriter(w, func(desc *format.Descriptor) (process, error) {
		p, err := r.LookupProcess(ctx, desc.Digest)
		if err != nil {
			return process{}, err
		}
		return process{
			ID:        p.ID,
			StartTime: human.Time(p.StartTime),
		}, nil
	})
}

func getRuntimes(ctx context.Context, w io.Writer, r *timemachine.Registry) stream.WriteCloser[*format.Descriptor] {
	type runtime struct {
		ID      string `text:"RUNTIME ID"`
		Runtime string `text:"RUNTIME NAME"`
		Version string `text:"VERSION"`
	}
	return newDescTableWriter(w, func(desc *format.Descriptor) (runtime, error) {
		r, err := r.LookupRuntime(ctx, desc.Digest)
		if err != nil {
			return runtime{}, err
		}
		return runtime{
			ID:      desc.Digest.Short(),
			Runtime: r.Runtime,
			Version: r.Version,
		}, nil
	})
}

func newDescTableWriter[T any](w io.Writer, conv func(*format.Descriptor) (T, error)) stream.WriteCloser[*format.Descriptor] {
	tw := textprint.NewTableWriter[T](w)
	cw := stream.ConvertWriter[T](tw, conv)
	return stream.NewWriteCloser(cw, tw)
}

func findResource(typ string, options []resource) (resource, bool) {
	for _, option := range options {
		if option.typ == typ {
			return option, true
		}
		for _, alt := range option.alt {
			if alt == typ {
				return option, true
			}
		}
	}
	return resource{}, false
}

func findMatchingResources(typ string, options []resource) (matches []resource) {
	for _, option := range options {
		if prefixLength(option.typ, typ) > 1 || prefixLength(typ, option.typ) > 1 {
			matches = append(matches, option)
		}
	}
	return matches
}

func prefixLength(base, prefix string) int {
	n := 0
	for n < len(base) && n < len(prefix) && base[n] == prefix[n] {
		n++
	}
	return n
}

func joinResourceTypes(resources []resource, prefix string) string {
	s := new(strings.Builder)
	for _, r := range resources {
		s.WriteString(prefix)
		s.WriteString(r.typ)
	}
	return s.String()
}

func useGet() string {
	s := new(strings.Builder)
	s.WriteString("\n\n")
	s.WriteString(`Use 'timecraft get <resource type>' where the supported resource types are:`)
	for _, r := range resources {
		s.WriteString("\n   ")
		s.WriteString(r.typ)
	}
	return s.String()
}
