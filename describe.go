package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	pprof "github.com/google/pprof/profile"
	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
	"github.com/stealthrocket/timecraft/internal/print/textprint"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
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

   $ timecraft describe log e1c6ae6e-caa8-45c1-bc9b-827617347063
   ID:      e1c6ae6e-caa8-45c1-bc9b-827617347063
   Size:    1.62 KiB/3.88 KiB +64 B (compression: 58.27%)
   Start:   3h ago, Mon, 29 May 2023 23:00:41 UTC
   Records: 27 (1 batch)
   ---
   SEGMENT  RECORDS  BATCHES  SIZE      UNCOMPRESSED SIZE  COMPRESSED SIZE  COMPRESSION RATIO
   0        27       1        1.68 KiB  3.88 KiB           1.62 KiB         58.27%

Options:
   -c, --config         Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -h, --help           Show this usage information
   -o, --output format  Output format, one of: text, json, yaml
`

func describe(ctx context.Context, args []string) error {
	output := outputFormat("text")

	flagSet := newFlagSet("timecraft describe", describeUsage)
	customVar(flagSet, &output, "o", "output")

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
		writer = textprint.NewWriter[any](os.Stdout)
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

	segments := reg.ListLogSegments(ctx, processID)
	defer segments.Close()

	i := stream.Iter[format.LogSegment](segments)
	for i.Next() {
		v := i.Value()
		desc.log = append(desc.log, logSegment{
			Number:    v.Number,
			Size:      human.Bytes(v.Size),
			CreatedAt: human.Time(v.CreatedAt.In(time.Local)),
		})
	}
	if err := i.Err(); err != nil {
		return nil, err
	}

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

func describeRecord(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	s := strings.Split(id, "/")
	if len(s) != 2 {
		return nil, errors.New(`record id should have the form <process id>/<offset>`)
	}
	pid, err := uuid.Parse(s[0])
	if err != nil {
		return nil, errors.New(`malformed process id (not a UUID)`)
	}
	offset, err := strconv.ParseInt(s[1], 10, 64)
	if err != nil {
		return nil, errors.New(`malformed record offset (not an integer)`)
	}

	r, err := reg.LookupRecord(ctx, pid, offset)
	if err != nil {
		return nil, err
	}

	rec := timemachine.Record{
		Time:         r.Time,
		FunctionID:   r.FunctionID,
		FunctionCall: r.FunctionCall,
	}

	dec := wasicall.Decoder{}
	_, syscall, _ := dec.Decode(rec)

	desc := &recordDescriptor{
		Record:  r,
		Syscall: syscall,
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
	return descriptorAndData(desc, proc), nil
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
	for _, env := range desc.env {
		fmt.Fprintf(w, "  %s\n", env)
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
	log       []logSegment
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
	for _, env := range desc.env {
		fmt.Fprintf(w, "  %s\n", env)
	}
	fmt.Fprintf(w, "Log:\n")
	for _, log := range desc.log {
		fmt.Fprintf(w, "  segment %d: %s, created %s (%s)\n", log.Number, log.Size, log.CreatedAt, time.Time(log.CreatedAt).Format(time.RFC1123))
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
	Number           int            `json:"number"        yaml:"number"        text:"SEGMENT"`
	NumRecords       int            `json:"-"             yaml:"-"             text:"RECORDS"`
	NumBatches       int            `json:"-"             yaml:"-"             text:"BATCHES"`
	Duration         human.Duration `json:"-"             yaml:"-"             text:"DURATION"`
	Size             human.Bytes    `json:"size"          yaml:"size"          text:"SIZE"`
	UncompressedSize human.Bytes    `json:"-"             yaml:"-"             text:"UNCOMPRESSED SIZE"`
	CompressedSize   human.Bytes    `json:"-"             yaml:"-"             text:"COMPRESSED SIZE"`
	CompressionRatio human.Ratio    `json:"-"             yaml:"-"             text:"COMPRESSION RATIO"`
	CreatedAt        human.Time     `json:"createdAt"     yaml:"createdAt"     text:"-"`
	RecordBatches    []recordBatch  `json:"recordBatches" yaml:"recordBatches" text:"-"`
}

func (desc *logSegment) Format(w fmt.State, _ rune) {
	startTime := human.Time{}
	if len(desc.RecordBatches) > 0 {
		startTime = desc.RecordBatches[0].FirstTimestamp
	}

	fmt.Fprintf(w, "Segment: %d\n", desc.Number)
	fmt.Fprintf(w, "Size:    %s/%s +%s (compression: %s)\n", desc.CompressedSize, desc.UncompressedSize, desc.Size-desc.CompressedSize, desc.CompressionRatio)
	fmt.Fprintf(w, "Start:   %s, %s\n", startTime, time.Time(startTime).Format(time.RFC1123))
	fmt.Fprintf(w, "Records: %d (%d batch)\n", desc.NumRecords, len(desc.RecordBatches))
	fmt.Fprintf(w, "---\n")

	table := textprint.NewTableWriter[recordBatch](w)
	defer table.Close()

	_, _ = table.Write(desc.RecordBatches)
}

type logDescriptor struct {
	ProcessID format.UUID  `json:"id"        yaml:"id"`
	Size      human.Bytes  `json:"size"      yaml:"size"`
	StartTime human.Time   `json:"startTime" yaml:"startTime"`
	Segments  []logSegment `json:"segments"  yaml:"segments"`
}

func (desc *logDescriptor) Format(w fmt.State, _ rune) {
	uncompressedSize := human.Bytes(0)
	compressedSize := human.Bytes(0)
	metadataSize := human.Bytes(0)
	numRecords := 0
	numBatches := 0
	for _, seg := range desc.Segments {
		uncompressedSize += seg.UncompressedSize
		compressedSize += seg.CompressedSize
		metadataSize += seg.Size - seg.CompressedSize
		numRecords += seg.NumRecords
		numBatches += seg.NumBatches
	}
	compressionRatio := 1 - human.Ratio(compressedSize)/human.Ratio(uncompressedSize)

	fmt.Fprintf(w, "ID:      %s\n", desc.ProcessID)
	fmt.Fprintf(w, "Size:    %s/%s +%s (compression: %s)\n", compressedSize, uncompressedSize, metadataSize, compressionRatio)
	fmt.Fprintf(w, "Start:   %s, %s\n", desc.StartTime, time.Time(desc.StartTime).Format(time.RFC1123))
	fmt.Fprintf(w, "Records: %d (%d batch)\n", numRecords, numBatches)
	fmt.Fprintf(w, "---\n")

	table := textprint.NewTableWriter[logSegment](w)
	defer table.Close()

	_, _ = table.Write(desc.Segments)
}

func describeLog(ctx context.Context, reg *timemachine.Registry, id string, config *configuration) (any, error) {
	logSegmentNumber := -1
	logID, logNumber, ok := strings.Cut(id, "/")
	if ok {
		n, err := strconv.Atoi(logNumber)
		if err != nil {
			return nil, errors.New(`malformed log id (suffix is not a valid segment number)`)
		}
		logSegmentNumber = n
	}

	processID, err := uuid.Parse(logID)
	if err != nil {
		return nil, errors.New(`malformed process id (not a UUID)`)
	}

	m, err := reg.LookupLogManifest(ctx, processID)
	if err != nil {
		return nil, err
	}

	logch := make(chan logSegment, len(m.Segments))
	errch := make(chan error, len(m.Segments))
	wait := 0

	for _, seg := range m.Segments {
		if logSegmentNumber >= 0 && seg.Number != logSegmentNumber {
			continue
		}
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
						logch <- logSegment
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

	var logs []logSegment
	var errs []error
	for wait > 0 {
		wait--
		select {
		case log := <-logch:
			logs = append(logs, log)
		case err := <-errch:
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return nil, errors.Join(errs...)
	}

	if logSegmentNumber >= 0 {
		return &logs[0], nil
	}

	slices.SortFunc(logs, func(s1, s2 logSegment) bool {
		return s1.Number < s2.Number
	})

	desc := &logDescriptor{
		ProcessID: processID,
		StartTime: human.Time(m.StartTime),
		Segments:  logs,
	}

	for _, log := range logs {
		desc.Size += log.Size
	}
	return desc, nil
}

type recordDescriptor struct {
	Record  format.Record    `json:"record"  yaml:"record"`
	Syscall wasicall.Syscall `json:"syscall" yaml:"syscall"`
}

func (desc *recordDescriptor) Format(w fmt.State, _ rune) {
	fmt.Fprintf(w, "Offset:  %d\n", desc.Record.Offset)
	fmt.Fprintf(w, "Segment: %d\n", desc.Record.Segment)
	fmt.Fprintf(w, "Process: %s\n", desc.Record.ProcessID)
	fmt.Fprintf(w, "Time:    %s\n", human.Time(desc.Record.Time))
	fmt.Fprintf(w, "Size:    %s\n", human.Bytes(len(desc.Record.FunctionCall)))
	fmt.Fprintf(w, "---\n")
	fmt.Fprintf(w, "Host function: %s\n", desc.Syscall.ID().String())
	fmt.Fprintf(w, "Parameters:")
	printParams(w, desc.Syscall.Params())
	fmt.Fprintf(w, "Results:")
	printParams(w, desc.Syscall.Results())
}

func printParams(w io.Writer, x []any) {
	if len(x) == 0 {
		fmt.Fprintf(w, " none\n")
	} else {
		fmt.Fprintf(w, " \n")
		for i, p := range x {
			fmt.Fprintf(w, "%d\ttype: ", i)
			printHumanType(w, reflect.TypeOf(p))
			fmt.Fprintf(w, "\tvalue: ")
			printHumanVal(w, p)
			fmt.Fprintf(w, "\n")
		}
	}
}

func printHumanType(w io.Writer, t reflect.Type) {
	switch t {
	case reflect.TypeOf(wasi.FD(0)):
		fmt.Fprintf(w, "fd")
		return
	case reflect.TypeOf(wasi.ClockID(0)):
		fmt.Fprintf(w, "clock")
		return
	case reflect.TypeOf(wasi.Timestamp(0)):
		fmt.Fprintf(w, "time")
		return
	case reflect.TypeOf(wasi.IOVec(nil)):
		fmt.Fprintf(w, "io")
		return
	case reflect.TypeOf(wasi.Errno(0)):
		fmt.Fprintf(w, "errno")
		return
	case reflect.TypeOf(wasi.Size(0)):
		fmt.Fprintf(w, "size")
		return
	case reflect.TypeOf(wasi.PreStat{}):
		fmt.Fprintf(w, "prestat")
		return
	}

	if t.Kind() == reflect.Slice {
		fmt.Fprintf(w, "[")
		printHumanType(w, t.Elem())
		fmt.Fprintf(w, "]")
		return
	}

	n := t.Name()
	if n == "" {
		n = t.String()
	}
	fmt.Fprintf(w, "%s", n)
}

func printHumanVal(w io.Writer, x any) {
	switch v := x.(type) {
	case wasi.FD, wasi.ExitCode:
		fmt.Fprintf(w, "%d", v)
	case wasi.Errno:
		fmt.Fprintf(w, "%d (%s)", v, v)
	case wasi.Size:
		fmt.Fprintf(w, "%d (%s)", v, human.Bytes(v))
	case []wasi.IOVec:
		fmt.Fprintf(w, "\n")
		for _, x := range v {
			o := hex.Dumper(prefixlines(nolastline(w), []byte("    | ")))
			_, _ = o.Write(x)
			o.Close()
		}
	default:
		fmt.Fprintf(w, "%s", x)
	}
}

func printHumanTypes(w io.Writer, x []any) {
	for i, t := range x {
		if i > 0 {
			fmt.Fprintf(w, ",")
		}
		printHumanType(w, reflect.TypeOf(t))
	}
}

type lineprefixer struct {
	p []byte
	w io.Writer

	start bool
}

func prefixlines(w io.Writer, prefix []byte) io.Writer {
	return &lineprefixer{
		p: prefix,
		w: w,

		start: true,
	}
}

func (l *lineprefixer) Write(b []byte) (int, error) {
	count := 0
	for len(b) > 0 {
		if l.start {
			l.start = false
			n, err := l.w.Write(l.p)
			count += n
			if err != nil {
				return count, err
			}
		}

		i := bytes.IndexByte(b, '\n')
		if i == -1 {
			i = len(b)
		} else {
			i++ // include \n
			l.start = true
		}
		n, err := l.w.Write(b[:i])
		count += n
		if err != nil {
			return count, err
		}
		b = b[i:]
	}
	return count, nil
}

func nolastline(w io.Writer) io.Writer {
	return &nolastliner{w: w}
}

type nolastliner struct {
	w   io.Writer
	has bool
}

func (l *nolastliner) Write(b []byte) (int, error) {
	if len(b) == 0 {
		return 0, nil
	}
	count := 0
	if l.has {
		l.has = false
		n, err := l.w.Write([]byte{'\n'})
		count += n
		if err != nil {
			return count, err
		}
	}
	i := bytes.LastIndexByte(b, '\n')
	if i == len(b)-1 {
		l.has = true
		b = b[:i]
	}
	n, err := l.w.Write(b)
	count += n
	return count, err
}
