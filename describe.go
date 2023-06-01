package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	pprof "github.com/google/pprof/profile"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
	"github.com/stealthrocket/timecraft/internal/print/textprint"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/tetratelabs/wazero/api"
	"golang.org/x/exp/slices"
)

const describeUsage = `
Usage:	timecraft describe <resource type> <resource ids...> [options]

   The describe command prints detailed information about specific resources.

   Resource types available to 'timecraft get' are also usable with the describe
   command. Values displayed in the ID column of the get output can be passed as
   arguments to describe.

Examples:

   $ timecraft describe runtime 79004e7beff2
   ID:      sha256:79004e7beff2db76e4fab5fdff04618bb7328829bcddb856462194994c8e46f9
   Runtime: timecraft
   Version: devel

Options:
   -c, --config path    Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -h, --help           Show this usage information
   -o, --output format  Output format, one of: text, json, yaml
   -v, --verbose        For text output, display more details about the resource
`

func describe(ctx context.Context, args []string) error {
	var (
		output  = outputFormat("text")
		verbose = false
	)

	flagSet := newFlagSet("timecraft describe", describeUsage)
	customVar(flagSet, &output, "o", "output")
	boolVar(flagSet, &verbose, "v", "verbose")

	args, err := parseFlags(flagSet, args)
	if err != nil {
		return err
	}
	if len(args) == 0 {
		perror(`expected a resource type as argument`)
		return exitCode(2)
	}

	resource, err := findResource("describe", args[0])
	if err != nil {
		return err
	}
	resourceIDs := args[1:]
	if len(resourceIDs) == 0 {
		return fmt.Errorf(`no resources were specified, use 'timecraft describe %s <resources ids...>'`, resource.typ)
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	registry, err := config.openRegistry()
	if err != nil {
		return err
	}

	format := "%v"
	if verbose {
		format = "%+v"
	}

	var lookup func(context.Context, *timemachine.Registry, string, *configuration) (any, error)
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
		writer = textprint.NewWriter[any](os.Stdout, textprint.Format[any](format))
	}
	defer writer.Close()

	readers := make([]stream.Reader[any], len(resourceIDs))
	for i := range resourceIDs {
		resource := resourceIDs[i]
		readers[i] = stream.ReaderFunc(func(values []any) (int, error) {
			v, err := lookup(ctx, registry, resource, config)
			if err != nil {
				return 0, err
			}
			values[0] = v
			return 1, io.EOF
		})
	}

	_, err = stream.Copy[any](writer, stream.MultiReader[any](readers...))
	return err
}

func describeConfig(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
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
		id: d.Digest.String(),
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
			id:   module.Digest.String(),
			name: moduleName(module),
			size: human.Bytes(module.Size),
		}
	}
	return desc, nil
}

func describeModule(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	d, err := reg.LookupDescriptor(ctx, format.ParseHash(id))
	if err != nil {
		return nil, err
	}
	m, err := reg.LookupModule(ctx, d.Digest)
	if err != nil {
		return nil, err
	}

	runtime := config.newRuntime(ctx)
	defer runtime.Close(ctx)

	compiledModule, err := runtime.CompileModule(ctx, m.Code)
	if err != nil {
		return nil, err
	}
	defer compiledModule.Close(ctx)

	desc := &moduleDescriptor{
		id:   d.Digest.String(),
		name: moduleName(d),
		size: human.Bytes(len(m.Code)),
	}

	for _, importedFunction := range compiledModule.ImportedFunctions() {
		moduleName, name, _ := importedFunction.Import()
		desc.imports.functions = append(desc.imports.functions, makeFunctionDefinition(moduleName, name, importedFunction))
	}

	for _, importedMemory := range compiledModule.ImportedMemories() {
		moduleName, name, _ := importedMemory.Import()
		desc.imports.memories = append(desc.imports.memories, makeMemoryDefinition(moduleName, name, importedMemory))
	}

	for name, exportedFunction := range compiledModule.ExportedFunctions() {
		desc.exports.functions = append(desc.exports.functions, makeFunctionDefinition(desc.name, name, exportedFunction))
	}

	for name, exportedMemory := range compiledModule.ExportedMemories() {
		desc.exports.memories = append(desc.exports.memories, makeMemoryDefinition(desc.name, name, exportedMemory))
	}

	sortFunctionDefinitions := func(f1, f2 functionDefinition) bool {
		if f1.Module != f2.Module {
			return f1.Module < f2.Module
		}
		return f1.Name < f2.Name
	}

	sortMemoryDefinitions := func(m1, m2 memoryDefinition) bool {
		if m1.Module != m2.Module {
			return m1.Module < m2.Module
		}
		return m1.Name < m2.Name
	}

	slices.SortFunc(desc.imports.functions, sortFunctionDefinitions)
	slices.SortFunc(desc.imports.memories, sortMemoryDefinitions)
	slices.SortFunc(desc.exports.functions, sortFunctionDefinitions)
	slices.SortFunc(desc.exports.memories, sortMemoryDefinitions)
	return desc, nil
}

func describeProcess(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
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
			id:   module.Digest.String(),
			name: moduleName(module),
			size: human.Bytes(module.Size),
		}
	}

	log, err := describeLog(ctx, reg, processID)
	if err != nil {
		return nil, err
	}
	desc.log = log
	return desc, nil
}

func describeProfile(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	d, err := reg.LookupDescriptor(ctx, format.ParseHash(id))
	if err != nil {
		return nil, err
	}
	p, err := reg.LookupProfile(ctx, d.Digest)
	if err != nil {
		return nil, err
	}
	desc := &profileDescriptor{
		id:          d.Digest.String(),
		processID:   d.Annotations["timecraft.process.id"],
		profileType: d.Annotations["timecraft.profile.type"],
		profile:     p,
	}
	return desc, nil
}

func describeRuntime(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	d, err := reg.LookupDescriptor(ctx, format.ParseHash(id))
	if err != nil {
		return nil, err
	}
	r, err := reg.LookupRuntime(ctx, d.Digest)
	if err != nil {
		return nil, err
	}
	desc := &runtimeDescriptor{
		id:      d.Digest.String(),
		runtime: r.Runtime,
		version: r.Version,
	}
	return desc, nil
}

func lookupConfig(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	return lookup(ctx, reg, id, (*timemachine.Registry).LookupConfig)
}

func lookupModule(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	return lookup(ctx, reg, id, (*timemachine.Registry).LookupModule)
}

func lookupProcess(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	_, desc, proc, err := lookupProcessByLogID(ctx, reg, id)
	if err != nil {
		return nil, err
	}
	log, err := describeLog(ctx, reg, proc.ID)
	if err != nil {
		return nil, err
	}
	return &struct {
		Desc     *format.Descriptor `json:"descriptor" yaml:"descriptor"`
		Data     *format.Process    `json:"data"       yaml:"data"`
		Segments []logSegment       `json:"segments"   yaml:"segments"`
	}{
		Desc:     desc,
		Data:     proc,
		Segments: log,
	}, nil
}

func lookupProfile(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	return lookup(ctx, reg, id, (*timemachine.Registry).LookupProfile)
}

func lookupRuntime(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
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
	processID, err := parseProcessID(id)
	if err != nil {
		return processID, nil, nil, err
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
	fmt.Fprintf(w, "ID:      %s\n", desc.id)
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
	if w.Flag('+') {
		for _, env := range desc.env {
			fmt.Fprintf(w, "  %s\n", env)
		}
	} else {
		fmt.Fprintf(w, "  ...\n")
	}
}

type moduleDescriptor struct {
	id      string
	name    string
	size    human.Bytes
	imports moduleDefinitions
	exports moduleDefinitions
}

type moduleDefinitions struct {
	functions []functionDefinition
	memories  []memoryDefinition
}

type functionDefinition struct {
	Section string `text:"SECTION"`
	Module  string `text:"MODULE"`
	Name    string `text:"FUNCTION"`
	Type    string `text:"SIGNATURE"`
}

func makeFunctionDefinition(module, name string, def api.FunctionDefinition) functionDefinition {
	s := new(strings.Builder)
	s.WriteString("(func")

	writeSignature := func(keyword string, names []string, types []api.ValueType) {
		for i := range types {
			s.WriteString(" (")
			s.WriteString(keyword)
			if i < len(names) {
				s.WriteString(" $")
				s.WriteString(names[i])
			}
			s.WriteString(" ")
			s.WriteString(api.ValueTypeName(types[i]))
			s.WriteString(")")
		}
	}

	writeSignature("param", def.ParamNames(), def.ParamTypes())
	writeSignature("result", def.ResultNames(), def.ResultTypes())
	s.WriteString(")")

	section := "import"
	if export := def.ExportNames(); len(export) > 0 {
		section = "export"
		name = strings.Join(export, ", ")
	}

	return functionDefinition{
		Section: section,
		Module:  module,
		Name:    name,
		Type:    s.String(),
	}
}

type memoryDefinition struct {
	Section string      `text:"SECTION"`
	Module  string      `text:"MODULE"`
	Name    string      `text:"MEMORY"`
	Min     human.Bytes `text:"MIN SIZE"`
	Max     human.Bytes `text:"MAX SIZE"`
}

func makeMemoryDefinition(module, name string, def api.MemoryDefinition) memoryDefinition {
	const pageSize = 65536
	max, hasMax := def.Max()
	if !hasMax {
		max = math.MaxUint32
	} else {
		max *= pageSize
	}
	section := "import"
	if export := def.ExportNames(); len(export) > 0 {
		section = "export"
		name = strings.Join(export, ", ")
	}
	return memoryDefinition{
		Section: section,
		Module:  module,
		Name:    name,
		Min:     human.Bytes(def.Min() * pageSize),
		Max:     human.Bytes(max),
	}
}

func (desc *moduleDescriptor) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "ID:   %s\n", desc.id)
	fmt.Fprintf(w, "Name: %s\n", desc.name)
	fmt.Fprintf(w, "Size: %v\n", desc.size)

	if len(desc.imports.functions)+len(desc.exports.functions) > 0 {
		fmt.Fprintf(w, "\n")
		fw := textprint.NewTableWriter[functionDefinition](w)
		_, _ = fw.Write(desc.imports.functions)
		_, _ = fw.Write(desc.exports.functions)
		_ = fw.Close()
	}

	if len(desc.imports.memories)+len(desc.exports.memories) > 0 {
		fmt.Fprintf(w, "\n")
		mw := textprint.NewTableWriter[memoryDefinition](w)
		_, _ = mw.Write(desc.imports.memories)
		_, _ = mw.Write(desc.exports.memories)
		_ = mw.Close()
	}
}

type processDescriptor struct {
	id        format.UUID
	startTime human.Time
	runtime   runtimeDescriptor
	modules   []moduleDescriptor
	args      []string
	env       []string
	log       logDescriptor
}

func (desc *processDescriptor) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "ID:      %s\n", desc.id)
	fmt.Fprintf(w, "Start:   %s, %s\n", desc.startTime, time.Time(desc.startTime).Format(time.RFC1123))
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
	if w.Flag('+') {
		for _, env := range desc.env {
			fmt.Fprintf(w, "  %s\n", env)
		}
	} else {
		fmt.Fprintf(w, "  ...\n")
	}
	if desc.log != nil {
		if w.Flag('+') {
			desc.log.Format(w, 's')
			for _, seg := range desc.log {
				seg.Format(w, 'v')
			}
		} else {
			desc.log.Format(w, 'v')
		}
	}
}

type profileDescriptor struct {
	id          string
	processID   string
	profileType string
	profile     *pprof.Profile
}

func (desc *profileDescriptor) Format(w fmt.State, _ rune) {
	startTime := human.Time(time.Unix(0, desc.profile.TimeNanos))
	duration := human.Duration(desc.profile.DurationNanos)

	fmt.Fprintf(w, "ID:       %s\n", desc.id)
	fmt.Fprintf(w, "Type:     %s\n", desc.profileType)
	fmt.Fprintf(w, "Process:  %s\n", desc.processID)
	fmt.Fprintf(w, "Start:    %s, %s\n", startTime, time.Time(startTime).Format(time.RFC1123))
	fmt.Fprintf(w, "Duration: %s\n", duration)

	if period := desc.profile.Period; period != 0 {
		fmt.Fprintf(w, "Period:   %d %s/%s\n", period, desc.profile.PeriodType.Type, desc.profile.PeriodType.Unit)
	} else {
		fmt.Fprintf(w, "Period:   (none)\n")
	}

	fmt.Fprintf(w, "Samples:  %d\n", len(desc.profile.Sample))
	hasDefault := false
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	for i, sampleType := range desc.profile.SampleType {
		if i != 0 {
			fmt.Fprintf(tw, "\n")
		}
		fmt.Fprintf(tw, "- %s\t%s", sampleType.Type, sampleType.Unit)
		if sampleType.Type == desc.profile.DefaultSampleType {
			hasDefault = true
			fmt.Fprintf(tw, " (default)")
		}
	}
	if !hasDefault {
		fmt.Fprintf(tw, " (default)\n")
	} else {
		fmt.Fprintf(tw, "\n")
	}
	_ = tw.Flush()

	if comments := desc.profile.Comments; len(comments) == 0 {
		fmt.Fprintf(w, "Comments: (none)\n")
	} else {
		fmt.Fprintf(w, "Comments:\n")
		for _, comment := range comments {
			fmt.Fprintf(w, "%s\n", comment)
		}
	}
}

type runtimeDescriptor struct {
	id      string
	runtime string
	version string
}

func (desc *runtimeDescriptor) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "ID:      %s\n", desc.id)
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

type recordBatch struct {
	SegmentNumber    int            `json:"-"                yaml:"-"                text:"SEGMENT"`
	NumRecords       int            `json:"numRecords"       yaml:"numRecords"       text:"RECORDS"`
	FirstOffset      int64          `json:"firstOffset"      yaml:"firstOffset"      text:"FIRST OFFSET"`
	FirstTimestamp   human.Time     `json:"firstTimestamp"   yaml:"firstTimestamp"   text:"-"`
	LastTimestamp    human.Time     `json:"lastTimestamp"    yaml:"lastTimestamp"    text:"-"`
	Duration         human.Duration `json:"-"                yaml:"-"                text:"DURATION"`
	UncompressedSize human.Bytes    `json:"uncompressedSize" yaml:"uncompressedSize" text:"UNCOMPRESSED SIZE"`
	CompressedSize   human.Bytes    `json:"compressedSize"   yaml:"compressedSize"   text:"COMPRESSED SIZE"`
	CompressionRatio human.Ratio    `json:"-"                yaml:"-"                text:"COMPRESSION RATIO"`
	Compression      string         `json:"compression"      yaml:"compression"      text:"COMPRESSION"`
}

type logSegment struct {
	Number           int            `json:"-"             yaml:"-"             text:"SEGMENT"`
	NumRecords       int            `json:"-"             yaml:"-"             text:"RECORDS"`
	NumBatches       int            `json:"-"             yaml:"-"             text:"BATCHES"`
	Duration         human.Duration `json:"-"             yaml:"-"             text:"DURATION"`
	CreatedAt        human.Time     `json:"createdAt"     yaml:"createdAt"     text:"-"`
	Size             human.Bytes    `json:"size"          yaml:"size"          text:"SIZE"`
	UncompressedSize human.Bytes    `json:"-"             yaml:"-"             text:"UNCOMPRESSED SIZE"`
	CompressedSize   human.Bytes    `json:"-"             yaml:"-"             text:"COMPRESSED SIZE"`
	CompressionRatio human.Ratio    `json:"-"             yaml:"-"             text:"COMPRESSION RATIO"`
	RecordBatches    []recordBatch  `json:"recordBatches" yaml:"recordBatches" text:"-"`
}

func (desc *logSegment) Format(w fmt.State, _ rune) {
	table := textprint.NewTableWriter[recordBatch](w)
	defer table.Close()
	_, _ = table.Write(desc.RecordBatches)
}

type logDescriptor []logSegment

func (desc logDescriptor) Format(w fmt.State, v rune) {
	uncompressedSize := human.Bytes(0)
	compressedSize := human.Bytes(0)
	metadataSize := human.Bytes(0)
	numRecords := 0
	numBatches := 0
	for _, seg := range desc {
		uncompressedSize += seg.UncompressedSize
		compressedSize += seg.CompressedSize
		metadataSize += seg.Size - seg.CompressedSize
		numRecords += seg.NumRecords
		numBatches += seg.NumBatches
	}
	compressionRatio := 1 - human.Ratio(compressedSize)/human.Ratio(uncompressedSize)

	fmt.Fprintf(w, "Records: %d, %d batch(es), %d segment(s), %s/%s +%s (compression: %s)\n---\n",
		numRecords,
		numBatches,
		len(desc),
		compressedSize,
		uncompressedSize,
		metadataSize,
		compressionRatio)

	if v == 'v' {
		table := textprint.NewTableWriter[logSegment](w)
		defer table.Close()
		_, _ = table.Write(desc)
	}
}

func describeLog(ctx context.Context, reg *timemachine.Registry, processID format.UUID) (logDescriptor, error) {
	m, err := reg.LookupLogManifest(ctx, processID)
	if err != nil {
		return nil, err
	}

	segch := make(chan logSegment, len(m.Segments))
	errch := make(chan error, len(m.Segments))
	wait := 0

	for _, seg := range m.Segments {
		wait++
		go func(seg format.LogSegment) {
			r, err := reg.ReadLogSegment(ctx, processID, seg.Number)
			if err != nil {
				errch <- err
				return
			}
			defer r.Close()

			logReader := timemachine.NewLogReader(r, seg.CreatedAt)
			defer logReader.Close()

			logSegment := logSegment{
				Number:    seg.Number,
				Size:      human.Bytes(seg.Size),
				CreatedAt: human.Time(seg.CreatedAt),
			}

			lastTime := time.Time(logSegment.CreatedAt)
			for {
				b, err := logReader.ReadRecordBatch()
				if err != nil {
					if err != io.EOF {
						errch <- err
					} else {
						c := human.Ratio(logSegment.CompressedSize)
						u := human.Ratio(logSegment.UncompressedSize)
						logSegment.CompressionRatio = 1 - c/u
						logSegment.NumBatches = len(logSegment.RecordBatches)
						logSegment.Duration = human.Duration(lastTime.Sub(time.Time(logSegment.CreatedAt)))
						segch <- logSegment
					}
					break
				}

				var (
					numRecords       = b.NumRecords()
					firstOffset      = b.FirstOffset()
					firstTimestamp   = b.FirstTimestamp()
					lastTimestamp    = b.LastTimestamp()
					duration         = human.Duration(lastTimestamp.Sub(firstTimestamp))
					uncompressedSize = human.Bytes(b.UncompressedSize())
					compressedSize   = human.Bytes(b.CompressedSize())
					compression      = b.Compression()
				)

				lastTime = lastTimestamp
				logSegment.NumRecords += numRecords
				logSegment.CompressedSize += compressedSize
				logSegment.UncompressedSize += uncompressedSize
				logSegment.RecordBatches = append(logSegment.RecordBatches, recordBatch{
					SegmentNumber:    seg.Number,
					NumRecords:       numRecords,
					FirstOffset:      firstOffset,
					FirstTimestamp:   human.Time(firstTimestamp),
					LastTimestamp:    human.Time(lastTimestamp),
					Duration:         duration,
					UncompressedSize: uncompressedSize,
					CompressedSize:   compressedSize,
					CompressionRatio: 1 - human.Ratio(compressedSize)/human.Ratio(uncompressedSize),
					Compression:      compression.String(),
				})
			}
		}(seg)
	}

	var segs []logSegment
	var errs []error
	for wait > 0 {
		wait--
		select {
		case seg := <-segch:
			segs = append(segs, seg)
		case err := <-errch:
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	slices.SortFunc(segs, func(s1, s2 logSegment) bool {
		return s1.Number < s2.Number
	})

	return logDescriptor(segs), nil
}
