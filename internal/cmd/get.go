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
Usage:	timecraft get <resource> [options]

   The get sub-command gives access to the state of the time machine registry.
   The command must be followed by the name of resources to display, which must
   be one of config, module, process, or runtime (the command also accept plural
   and prefixes of the resource names).

Examples:

   $ timecraft get modules
   MODULE ID     MODULE NAME  SIZE
   a11c1643e362  (none)       6.82 MiB

   $ timecraft get modules -o json
   [
     {
       "mediaType": "application/vnd.timecraft.module.v1+wasm",
       "digest": "sha256:a11c1643e362e9fe0f0a40b45f0f2212e4a725ad8e969ce9fd7ff074cfe7e0d8",
       "size": 7150231,
       "annotations": {
         "timecraft.object.created-at": "2023-05-28T19:50:41Z",
         "timecraft.object.resource-type": "module"
       }
     }
   ]

Options:
   -h, --help           Show this usage information
   -o, --ouptut format  Output format, one of: text, json, yaml
   -r, --registry path  Path to the timecraft registry (default to ~/.timecraft)
`

type resource struct {
	name string
	alt  []string
	get  func(context.Context, io.Writer, *timemachine.Registry) stream.WriteCloser[*format.Descriptor]
}

var resources = [...]resource{
	{
		name: "config",
		alt:  []string{"configs"},
		get:  getConfigs,
	},
	{
		name: "module",
		alt:  []string{"mo", "mods", "modules"},
		get:  getModules,
	},
	{
		name: "process",
		alt:  []string{"ps", "procs", "processes"},
		get:  getProcesses,
	},
	{
		name: "runtime",
		alt:  []string{"rt", "runtimes"},
		get:  getRuntimes,
	},
}

func get(ctx context.Context, args []string) error {
	var (
		timeRange    = timemachine.Since(time.Unix(0, 0))
		output       = outputFormat("text")
		registryPath = "~/.timecraft"
	)

	flagSet := newFlagSet("timecraft get", getUsage)
	customVar(flagSet, &output, "o", "output")
	stringVar(flagSet, &registryPath, "r", "registry")
	parseFlags(flagSet, args)

	args = flagSet.Args()
	if len(args) == 0 {
		return errors.New(`expected exactly one resource name as argument`)
	}
	resourceNamePrefix := args[0]
	parseFlags(flagSet, args[1:])

	matchingResources := selectMatchingResources(resourceNamePrefix, resources[:])
	if len(matchingResources) == 0 {
		return fmt.Errorf(`no resources matching '%s'`, resourceNamePrefix)
	}
	if len(matchingResources) > 1 {
		return fmt.Errorf(`no resources matching '%s'

Did you mean?

    $ timecraft get %s
`, resourceNamePrefix, matchingResources[0].name)
	}

	registry, err := openRegistry(registryPath)
	if err != nil {
		return err
	}

	resource := matchingResources[0]
	reader := registry.ListResources(ctx, resource.name, timeRange)
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
			ID:      desc.Digest.Digest[:12],
			Runtime: r.Version,
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
			ID:   desc.Digest.Digest[:12],
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
		ID        string     `text:"RUNTIME ID"`
		Version   string     `text:"VERSION"`
		CreatedAt human.Time `text:"CREATED"`
	}
	return newDescTableWriter(w, func(desc *format.Descriptor) (runtime, error) {
		r, err := r.LookupRuntime(ctx, desc.Digest)
		if err != nil {
			return runtime{}, err
		}
		t, err := human.ParseTime(desc.Annotations["timecraft.object.created-at"])
		if err != nil {
			return runtime{}, err
		}
		return runtime{
			ID:        desc.Digest.Digest[:12],
			Version:   r.Version,
			CreatedAt: t,
		}, nil
	})
}

func newDescTableWriter[T any](w io.Writer, conv func(*format.Descriptor) (T, error)) stream.WriteCloser[*format.Descriptor] {
	tw := textprint.NewTableWriter[T](w)
	cw := stream.ConvertWriter[T](tw, conv)
	return stream.NewWriteCloser(cw, tw)
}

func selectMatchingResources(name string, options []resource) []resource {
	var matches []resource

	for _, option := range options {
		if option.name == name {
			return []resource{option}
		}
		for _, alt := range option.alt {
			if alt == name {
				return []resource{option}
			}
		}
		if strings.HasPrefix(option.name, name) {
			matches = append(matches, option)
		}
	}

	return matches
}
