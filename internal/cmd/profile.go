package cmd

import (
	"context"
	"errors"
	"io"
	"os"
	"time"

	pprof "github.com/google/pprof/profile"
	"github.com/google/uuid"

	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/print/human"
	"github.com/stealthrocket/timecraft/internal/print/jsonprint"
	"github.com/stealthrocket/timecraft/internal/print/yamlprint"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/stealthrocket/wzprof"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/experimental"
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

   $ timecraft profile f6e9acbc-0543-47df-9413-b99f569cfa3b
   ...

   $ go tool pprof -http :4040 cpu.out
   (web page opens in browser)

Options:
   -d, --duration duration  Amount of time that the profiler will be running for (default to the process up time)
   -h, --help               Show this usage information
   -o, --ouptut format      Output format, one of: text, json, yaml
   -t, --start-time time    Time at which the profiler gets started (default to 1 minute)
   -r, --registry path      Path to the timecraft registry (default to ~/.timecraft)
`

func profile(ctx context.Context, args []string) error {
	var (
		output       = outputFormat("text")
		startTime    = human.Time{}
		duration     = human.Duration(1 * time.Minute)
		registryPath = human.Path("~/.timecraft")
	)

	flagSet := newFlagSet("timecraft profile", profileUsage)
	customVar(flagSet, &output, "o", "output")
	customVar(flagSet, &duration, "d", "duration")
	customVar(flagSet, &startTime, "t", "start-time")
	customVar(flagSet, &registryPath, "r", "registry")
	args = parseFlags(flagSet, args)

	if len(args) != 1 {
		return errors.New(`expected exactly one process id as argument`)
	}

	processID, err := uuid.Parse(args[0])
	if err != nil {
		return errors.New(`malformed process id passed as argument (not a UUID)`)
	}

	registry, err := openRegistry(registryPath)
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
	config, err := registry.LookupConfig(ctx, process.Config.Digest)
	if err != nil {
		return err
	}
	module, err := registry.LookupModule(ctx, config.Modules[0].Digest)
	if err != nil {
		return err
	}

	logSegment, err := registry.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return err
	}
	defer logSegment.Close()

	logReader := timemachine.NewLogReader(logSegment, manifest.StartTime)
	defer logReader.Close()

	records := &recordProfiler{
		records:    timemachine.NewLogRecordReader(logReader),
		startTime:  timeRange.Start,
		endTime:    timeRange.End,
		sampleRate: 1.0,
	}
	records.cpu = wzprof.NewCPUProfiler(wzprof.TimeFunc(records.now))
	records.mem = wzprof.NewMemoryProfiler()

	ctx = context.WithValue(ctx,
		experimental.FunctionListenerFactoryKey{},
		experimental.MultiFunctionListenerFactory(
			wzprof.Flag(&records.started, records.cpu),
			wzprof.Flag(&records.started, records.mem),
		),
	)

	runtime := wazero.NewRuntime(ctx)
	defer runtime.Close(ctx)

	compiledModule, err := runtime.CompileModule(ctx, module.Code)
	if err != nil {
		return err
	}
	defer compiledModule.Close(ctx)

	replay := wasicall.NewReplay(records)
	defer replay.Close(ctx)

	system := wasicall.NewFallbackSystem(replay, wasicall.NewExitSystem(0))

	// TODO: need to figure this out dynamically:
	hostModule := wasi_snapshot_preview1.NewHostModule(wasi_snapshot_preview1.WasmEdgeV2)
	hostModuleInstance := wazergo.MustInstantiate(ctx, runtime, hostModule, wasi_snapshot_preview1.WithWASI(system))
	ctx = wazergo.WithModuleInstance(ctx, hostModuleInstance)

	if err := exec(ctx, runtime, compiledModule); err != nil {
		return err
	}

	records.stop()

	desc, err := createProfiles(registry, processID, records.cpuProfile, records.memProfile)
	if err != nil {
		return err
	}

	var writer stream.WriteCloser[*format.Descriptor]
	switch output {
	case "json":
		writer = jsonprint.NewWriter[*format.Descriptor](os.Stdout)
	case "yaml":
		writer = yamlprint.NewWriter[*format.Descriptor](os.Stdout)
	default:
		writer = getProfiles(ctx, os.Stdout, registry)
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

	firstTimestamp int64
	lastTimestamp  int64

	startTime  time.Time
	endTime    time.Time
	started    bool
	stopped    bool
	sampleRate float64
	symbols    wzprof.Symbolizer
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
		r.lastTimestamp = records[0].Timestamp()
		if !r.started && !records[0].Time().Before(r.startTime) {
			r.firstTimestamp = r.lastTimestamp
			r.start()
		}
		if !r.stopped && !records[0].Time().Before(r.endTime) {
			r.stop()
		}
	}
	return n, err
}

func (r *recordProfiler) now() int64 {
	return r.lastTimestamp
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
		r.cpuProfile = r.cpu.StopProfile(r.sampleRate, r.symbols)
		r.memProfile = r.mem.NewProfile(r.sampleRate, r.symbols)
		r.cpuProfile.TimeNanos = r.startTime.UnixNano()
		r.memProfile.TimeNanos = r.startTime.UnixNano()
		duration := r.lastTimestamp - r.firstTimestamp
		r.cpuProfile.DurationNanos = duration
		r.memProfile.DurationNanos = duration
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

	var descriptors = make([]*format.Descriptor, 0, len(profiles))
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
