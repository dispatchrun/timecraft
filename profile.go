package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	pprof "github.com/google/pprof/profile"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timecraft"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/stealthrocket/wzprof"
	"github.com/tetratelabs/wazero/experimental"
	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
)

const profileUsage = `
Usage:	timecraft profile [options] <process id>

   The profile command provides the ability to generate performance profiles
   from records of an execution timeline. The profiles can be scopped to a time
   range of interest and written to files in the format understood by pprof.

   For resources on how to use pprof, see:
   - https://go.dev/blog/pprof
   - https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/

Example:

   $ timecraft profile --export memory:mem.out f6e9acbc-0543-47df-9413-b99f569cfa3b
   ==> writing memory profile to mem.out
   ...

   $ go tool pprof -http :4040 cpu.out
   (web page opens in browser)

Options:
   -c, --config path        Path to the timecraft configuration file (overrides TIMECRAFTCONFIG)
   -d, --duration duration  Amount of time that the profiler will be running for (default to the process up time)
       --export type:path   Exports the generated profiles, type is one of cpu or memory (may be repeated)
   -h, --help               Show this usage information
   -o, --output format      Output format, one of: text, json, yaml
   -q, --quiet              Only display the profile ids
   -t, --start-time time    Time at which the profiler gets started (default to 1 minute)
`

func profile(ctx context.Context, args []string) error {
	var (
		exports   = stringMap{}
		output    = outputFormat("text")
		startTime = human.Time{}
		duration  = human.Duration(1 * time.Minute)
		quiet     = false
	)

	flagSet := newFlagSet("timecraft profile", profileUsage)
	customVar(flagSet, &exports, "export")
	customVar(flagSet, &output, "o", "output")
	customVar(flagSet, &duration, "d", "duration")
	customVar(flagSet, &startTime, "t", "start-time")
	boolVar(flagSet, &quiet, "q", "quiet")

	args, err := parseFlags(flagSet, args)
	if err != nil {
		return err
	}
	if len(args) != 1 {
		return errors.New(`expected exactly one process id as argument`)
	}

	exportedProfileTypes := maps.Keys(exports)
	slices.Sort(exportedProfileTypes)

	for _, typ := range exportedProfileTypes {
		switch typ {
		case "cpu", "memory":
		default:
			return fmt.Errorf(`unsupported profile type: %s`, typ)
		}
	}

	processID, err := parseProcessID(args[0])
	if err != nil {
		return err
	}
	config, err := timecraft.LoadConfig()
	if err != nil {
		return err
	}
	registry, err := config.OpenRegistry()
	if err != nil {
		return err
	}

	manifest, err := registry.LookupLogManifest(ctx, processID)
	if err != nil {
		return err
	}
	if startTime.IsZero() {
		startTime = human.Time(manifest.StartTime)
	}
	timeRange := timemachine.TimeRange{
		Start: time.Time(startTime),
		End:   time.Time(startTime).Add(time.Duration(duration)),
	}

	process, err := registry.LookupProcess(ctx, manifest.Process.Digest)
	if err != nil {
		return err
	}
	processConfig, err := registry.LookupConfig(ctx, process.Config.Digest)
	if err != nil {
		return err
	}
	module, err := registry.LookupModule(ctx, processConfig.Modules[0].Digest)
	if err != nil {
		return err
	}

	logSegment, err := registry.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return err
	}
	defer logSegment.Close()

	logReader := timemachine.NewLogReader(logSegment, manifest)
	defer logReader.Close()

	records := &recordProfiler{
		records:    timemachine.NewLogRecordReader(logReader),
		startTime:  timeRange.Start,
		endTime:    timeRange.End,
		sampleRate: 1.0,
	}
	p := wzprof.ProfilingFor(module.Code)
	// Enable profiling of time spent in host functions because we don't have
	// any I/O wait during a replay, so it gives a useful perspective of the
	// CPU time spent processing the host call invocation.
	records.cpu = p.CPUProfiler(wzprof.HostTime(true))
	records.mem = p.MemoryProfiler()

	ctx = context.WithValue(ctx,
		experimental.FunctionListenerFactoryKey{},
		experimental.MultiFunctionListenerFactory(
			wzprof.Flag(&records.started, records.cpu),
			wzprof.Flag(&records.started, records.mem),
		),
	)

	runtime, err := config.NewRuntime(ctx)
	if err != nil {
		return err
	}
	defer runtime.Close(ctx)

	compiledModule, err := runtime.CompileModule(ctx, module.Code)
	if err != nil {
		return err
	}
	defer compiledModule.Close(ctx)
	if err := p.Prepare(compiledModule); err != nil {
		return err
	}

	replay := wasicall.NewReplay(records)
	defer replay.Close(ctx)

	system := wasicall.NewFallbackSystem(replay, wasicall.NewExitSystem(0))

	hostModule := wasi_snapshot_preview1.NewHostModule(imports.DetectExtensions(compiledModule)...)
	hostModuleInstance := wazergo.MustInstantiate(ctx, runtime, hostModule, wasi_snapshot_preview1.WithWASI(system))
	ctx = wazergo.WithModuleInstance(ctx, hostModuleInstance)

	if err := instantiate(ctx, runtime, compiledModule); err != nil {
		return err
	}

	records.stop()

	desc, err := createProfiles(registry, processID, records.cpuProfile, records.memProfile)
	if err != nil {
		return err
	}

	for _, typ := range exportedProfileTypes {
		var p *pprof.Profile
		switch typ {
		case "cpu":
			p = records.cpuProfile
		case "memory":
			p = records.memProfile
		}
		if p != nil {
			path := exports[typ]
			perrorf("==> writing %s profile to %s", typ, path)
			if err := wzprof.WriteProfile(path, p); err != nil {
				return err
			}
		}
	}

	var writer stream.WriteCloser[*format.Descriptor]
	switch output {
	case "json":
		writer = jsonprint.NewWriter[*format.Descriptor](os.Stdout)
	case "yaml":
		writer = yamlprint.NewWriter[*format.Descriptor](os.Stdout)
	default:
		writer = getProfiles(ctx, os.Stdout, registry, quiet)
	}
	defer writer.Close()

	_, err = stream.Copy[*format.Descriptor](writer, stream.NewReader(desc...))
	return err
}

type recordProfiler struct {
	records stream.Reader[timemachine.Record]

	cpu *wzprof.CPUProfiler
	mem *wzprof.MemoryProfiler

	cpuProfile *pprof.Profile
	memProfile *pprof.Profile

	firstTimestamp time.Time
	lastTimestamp  time.Time

	startTime  time.Time
	endTime    time.Time
	started    bool
	stopped    bool
	sampleRate float64
}

func (r *recordProfiler) Read(records []timemachine.Record) (int, error) {
	if len(records) == 0 {
		return 0, nil
	}
	if r.stopped {
		return 0, io.EOF
	}
	n, err := r.records.Read(records[:1])
	if n > 0 {
		r.lastTimestamp = records[0].Time
		if !r.started && !r.lastTimestamp.Before(r.startTime) {
			r.firstTimestamp = r.lastTimestamp
			r.start()
		}
		if !r.stopped && !r.lastTimestamp.Before(r.endTime) {
			r.stop()
		}
	}
	return n, err
}

func (r *recordProfiler) start() {
	if !r.started {
		r.started = true
		r.cpu.StartProfile()
	}
}

func (r *recordProfiler) stop() {
	if !r.stopped {
		r.stopped = true
		r.cpuProfile = r.cpu.StopProfile(r.sampleRate)
		r.memProfile = r.mem.NewProfile(r.sampleRate)
		r.cpuProfile.TimeNanos = r.startTime.UnixNano()
		r.memProfile.TimeNanos = r.startTime.UnixNano()
		duration := r.lastTimestamp.Sub(r.firstTimestamp)
		r.cpuProfile.DurationNanos = int64(duration)
		r.memProfile.DurationNanos = int64(duration)
	}
}

func createProfiles(reg *timemachine.Registry, processID format.UUID, profiles ...*pprof.Profile) ([]*format.Descriptor, error) {
	mapping := []*pprof.Mapping{{
		ID:   1,
		File: "module.wasm",
	}}

	ch := make(chan stream.Optional[*format.Descriptor])
	for _, p := range profiles {
		p.Mapping = mapping

		profileType := "memory"
		for _, sample := range p.SampleType {
			if sample.Type == "cpu" || sample.Type == "samples" {
				profileType = "cpu"
				break
			}
		}

		go func(profile *pprof.Profile) {
			ch <- stream.Opt(reg.CreateProfile(context.TODO(), processID, profileType, profile))
		}(p)
	}

	descriptors := make([]*format.Descriptor, 0, len(profiles))
	var lastErr error
	for range profiles {
		d, err := (<-ch).Value()
		if err != nil {
			lastErr = err
		} else {
			descriptors = append(descriptors, d)
		}
	}
	return descriptors, lastErr
}
