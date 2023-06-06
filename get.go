package main

import (
	"bytes"
	"context"
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
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
)

const getUsage = `
Usage:	timecraft get <resource type> [options]

   The get sub-command gives access to the state of the time machine registry.
   The command must be followed by the name of resources to display, which must
   be one of config, module, process, profile, or runtime.
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
       "timecraft.module.name": "app.wasm"
     }
   }

Options:
   -c, --config path    Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -h, --help           Show this usage information
   -o, --output format  Output format, one of: text, json, yaml
   -q, --quiet          Only display the resource ids
`

type resource struct {
	typ       string
	alt       []string
	mediaType format.MediaType
	get       func(context.Context, io.Writer, *timemachine.Registry, bool) stream.WriteCloser[*format.Descriptor]
	describe  func(context.Context, *timemachine.Registry, string, *configuration) (any, error)
	lookup    func(context.Context, *timemachine.Registry, string, *configuration) (any, error)
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
		describe:  describeProcess,
		lookup:    lookupProcess,
	},

	{
		typ:       "profile",
		alt:       []string{"prof", "profs", "profiles"},
		mediaType: format.TypeTimecraftProfile,
		get:       getProfiles,
		describe:  describeProfile,
		lookup:    lookupProfile,
	},

	{
		typ:      "record",
		alt:      []string{"rec", "recs", "records"},
		describe: describeRecord,
		lookup:   lookupRecord,
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
		timeRange = timemachine.Since(time.Unix(0, 0))
		output    = outputFormat("text")
		quiet     = false
	)

	flagSet := newFlagSet("timecraft get", getUsage)
	customVar(flagSet, &output, "o", "output")
	boolVar(flagSet, &quiet, "q", "quiet")

	args, err := parseFlags(flagSet, args)
	if err != nil {
		return err
	}
	if len(args) < 1 {
		perrorf(`Expected at least the resource type as argument` + useCmd("get"))
		return exitCode(2)
	}
	resource, err := findResource("get", args[0])
	if err != nil {
		perror(err)
		return exitCode(2)
	}
	args = args[1:]
	config, err := loadConfig()
	if err != nil {
		return err
	}
	registry, err := config.openRegistry()
	if err != nil {
		return err
	}

	// We make a special case for the processes segments because they are not
	// immutable objects, some of the details we can print about processes
	// come from reading the log segments.
	if resource.typ == "process" {
		reader := registry.ListLogManifests(ctx) // TODO: time range
		defer reader.Close()

		var writer stream.WriteCloser[*format.Manifest]
		switch output {
		case "json":
			writer = jsonprint.NewWriter[*format.Manifest](os.Stdout)
		case "yaml":
			writer = yamlprint.NewWriter[*format.Manifest](os.Stdout)
		default:
			writer = getProcesses(ctx, os.Stdout, registry, quiet)
		}
		defer writer.Close()

		_, err = stream.Copy[*format.Manifest](writer, reader)
		return err
	}

	if resource.typ == "record" {
		if len(args) != 1 {
			perrorf(`Expected the process id as argument` + useCmd("get records"))
		}

		processID, err := uuid.Parse(args[0])
		if err != nil {
			return err
		}

		manifest, err := registry.LookupLogManifest(ctx, processID)
		if err != nil {
			return err
		}

		reader := registry.ListRecords(ctx, processID, timeRange)
		defer reader.Close()

		decoded := recordsDecoder(manifest, reader)

		var writer stream.WriteCloser[format.Record]
		switch output {
		case "json":
			writer = jsonprint.NewWriter[format.Record](os.Stdout)
		case "yaml":
			writer = yamlprint.NewWriter[format.Record](os.Stdout)
		default:
			writer = getRecords(ctx, os.Stdout, registry, quiet)
		}
		defer writer.Close()
		_, err = stream.Copy[format.Record](writer, decoded)

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
		writer = resource.get(ctx, os.Stdout, registry, quiet)
	}
	defer writer.Close()

	_, err = stream.Copy[*format.Descriptor](writer, reader)
	return err
}

func recordsDecoder(manifest *format.Manifest, from stream.Reader[timemachine.Record]) stream.Reader[format.Record] {
	dec := wasicall.Decoder{}
	return stream.ConvertReader[format.Record, timemachine.Record](from, func(x timemachine.Record) (format.Record, error) {
		out := format.Record{
			ID:      fmt.Sprintf("%s/%d", manifest.ProcessID, x.Offset),
			Process: manifest.Process,
			Offset:  x.Offset,
			Size:    int64(len(x.FunctionCall)),
			Time:    x.Time,
		}
		_, syscall, err := dec.Decode(x)
		if err == nil {
			out.Function = syscall.ID().String()
		} else {
			out.Function = fmt.Sprintf("%d (ERR: %s)", x.FunctionID, err)
		}
		return out, nil
	})
}

func getConfigs(ctx context.Context, w io.Writer, reg *timemachine.Registry, quiet bool) stream.WriteCloser[*format.Descriptor] {
	type config struct {
		ID      string      `text:"CONFIG ID"`
		Runtime string      `text:"RUNTIME"`
		Modules int         `text:"MODULES"`
		Size    human.Bytes `text:"SIZE"`
	}
	return newTableWriter(w, quiet,
		func(c1, c2 config) bool {
			return c1.ID < c2.ID
		},
		func(desc *format.Descriptor) (config, error) {
			c, err := reg.LookupConfig(ctx, desc.Digest)
			if err != nil {
				return config{}, err
			}
			r, err := reg.LookupRuntime(ctx, c.Runtime.Digest)
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

func getModules(ctx context.Context, w io.Writer, reg *timemachine.Registry, quiet bool) stream.WriteCloser[*format.Descriptor] {
	type module struct {
		ID   string      `text:"MODULE ID"`
		Name string      `text:"MODULE NAME"`
		Size human.Bytes `text:"SIZE"`
	}
	return newTableWriter(w, quiet,
		func(m1, m2 module) bool {
			return m1.ID < m2.ID
		},
		func(desc *format.Descriptor) (module, error) {
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

func getProcesses(ctx context.Context, w io.Writer, reg *timemachine.Registry, quiet bool) stream.WriteCloser[*format.Manifest] {
	type manifest struct {
		ProcessID format.UUID `text:"PROCESS ID"`
		StartTime human.Time  `text:"START"`
		Size      human.Bytes `text:"SIZE"`
	}
	return newTableWriter(w, quiet,
		func(m1, m2 manifest) bool {
			return time.Time(m1.StartTime).Before(time.Time(m2.StartTime))
		},
		func(m *format.Manifest) (manifest, error) {
			manifest := manifest{
				ProcessID: m.ProcessID,
				StartTime: human.Time(m.StartTime),
			}
			for _, segment := range m.Segments {
				manifest.Size += human.Bytes(segment.Size)
			}
			return manifest, nil
		})
}

func getProfiles(ctx context.Context, w io.Writer, reg *timemachine.Registry, quiet bool) stream.WriteCloser[*format.Descriptor] {
	type profile struct {
		ID        string         `text:"PROFILE ID"`
		ProcessID format.UUID    `text:"PROCESS ID"`
		Type      string         `text:"TYPE"`
		StartTime human.Time     `text:"START"`
		Duration  human.Duration `text:"DURATION"`
		Size      human.Bytes    `text:"SIZE"`
	}
	return newTableWriter(w, quiet,
		func(p1, p2 profile) bool {
			if p1.ProcessID != p2.ProcessID {
				return bytes.Compare(p1.ProcessID[:], p2.ProcessID[:]) < 0
			}
			if p1.Type != p2.Type {
				return p1.Type < p2.Type
			}
			if !time.Time(p1.StartTime).Equal(time.Time(p2.StartTime)) {
				return time.Time(p1.StartTime).Before(time.Time(p2.StartTime))
			}
			return p1.Duration < p2.Duration
		},
		func(desc *format.Descriptor) (profile, error) {
			processID, _ := uuid.Parse(desc.Annotations["timecraft.process.id"])
			startTime, _ := time.Parse(time.RFC3339Nano, desc.Annotations["timecraft.profile.start"])
			endTime, _ := time.Parse(time.RFC3339Nano, desc.Annotations["timecraft.profile.end"])
			return profile{
				ID:        desc.Digest.Short(),
				ProcessID: processID,
				Type:      desc.Annotations["timecraft.profile.type"],
				StartTime: human.Time(startTime),
				Duration:  human.Duration(endTime.Sub(startTime)),
				Size:      human.Bytes(desc.Size),
			}, nil
		})
}

func getRuntimes(ctx context.Context, w io.Writer, reg *timemachine.Registry, quiet bool) stream.WriteCloser[*format.Descriptor] {
	type runtime struct {
		ID      string `text:"RUNTIME ID"`
		Runtime string `text:"RUNTIME NAME"`
		Version string `text:"VERSION"`
	}
	return newTableWriter(w, quiet,
		func(r1, r2 runtime) bool {
			return r1.ID < r2.ID
		},
		func(desc *format.Descriptor) (runtime, error) {
			r, err := reg.LookupRuntime(ctx, desc.Digest)
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

func getRecords(ctx context.Context, w io.Writer, reg *timemachine.Registry, quiet bool) stream.WriteCloser[format.Record] {
	type record struct {
		ID      string      `text:"RECORD ID"`
		Time    human.Time  `text:"TIME"`
		Size    human.Bytes `text:"SIZE"`
		Syscall string      `text:"HOST FUNCTION"`
	}
	return newTableWriter(w, quiet, nil,
		func(r format.Record) (record, error) {
			out := record{
				ID:      r.ID,
				Time:    human.Time(r.Time),
				Size:    human.Bytes(r.Size),
				Syscall: r.Function,
			}
			return out, nil
		},
	)
}

func newTableWriter[T1, T2 any](w io.Writer, quiet bool, orderBy func(T1, T1) bool, conv func(T2) (T1, error)) stream.WriteCloser[T2] {
	opts := []textprint.TableOption[T1]{
		textprint.OrderBy(orderBy),
	}
	if quiet {
		opts = append(opts,
			textprint.Header[T1](false),
			textprint.List[T1](true),
		)
	}
	tw := textprint.NewTableWriter[T1](w, opts...)
	cw := stream.ConvertWriter[T1](tw, conv)
	return stream.NewWriteCloser(cw, tw)
}

func findResource(cmd, typ string) (*resource, error) {
	for i, resource := range resources {
		if resource.typ == typ {
			return &resources[i], nil
		}
		for _, alt := range resource.alt {
			if alt == typ {
				return &resources[i], nil
			}
		}
	}

	var matchingResources []*resource
	for i, resource := range resources {
		if prefixLength(resource.typ, typ) > 1 || prefixLength(typ, resource.typ) > 1 {
			matchingResources = append(matchingResources, &resources[i])
		}
	}
	if len(matchingResources) == 0 {
		return nil, fmt.Errorf(`no resources matching '%s'%s`, typ, useCmd(cmd))
	}

	var resourceTypes strings.Builder
	for _, r := range matchingResources {
		resourceTypes.WriteString("\n  ")
		resourceTypes.WriteString(r.typ)
	}

	return nil, fmt.Errorf("no resources matching '%s'\n\nDid you mean?%s", typ, &resourceTypes)
}

func prefixLength(base, prefix string) int {
	n := 0
	for n < len(base) && n < len(prefix) && base[n] == prefix[n] {
		n++
	}
	return n
}

func useCmd(cmd string) string {
	s := new(strings.Builder)
	s.WriteString("\n\n")
	s.WriteString(`Use 'timecraft ` + cmd + ` <resource type>' where the supported resource types are:`)
	for _, r := range resources {
		s.WriteString("\n   ")
		s.WriteString(r.typ)
	}
	return s.String()
}
