package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"time"

	pprof "github.com/google/pprof/profile"
	"github.com/google/uuid"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go/imports"
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
    writing cpu profile:	cpu.out
    writing memory profile:	mem.out

    $ go tool pprof -http :4040 cpu.out
    (web page opens in browser)

Options:
       --cpuprofile path    Path where the CPU profile will be written (default to cpu.out)
       --duration duration  Amount of time that the profiler will be running for (default to the process up time)
   -h, --help               Show this usage information
       --memprofile path    Path where the memory profile will be written (default to mem.out)
       --sample-rate ratio  Ratio of function calls recorded by the profiler, expressed as a decimal number between 0 and 1 (default to 1)
       --start-time time    Time at which the profiler gets started (default to the process start time)
   -r, --registry path      Path to the timecraft registry (default to ~/.timecraft)
`

func profile(ctx context.Context, args []string) error {
	var (
		startTime    timestamp
		duration     time.Duration
		sampleRate   = 1.0
		cpuProfile   = "cpu.out"
		memProfile   = "mem.out"
		registryPath = "~/.timecraft"
	)

	flagSet := newFlagSet("timecraft profile", profileUsage)
	customVar(flagSet, &startTime, "start-time")
	durationVar(flagSet, &duration, "duration")
	float64Var(flagSet, &sampleRate, "sample-rate")
	stringVar(flagSet, &cpuProfile, "cpuprofile")
	stringVar(flagSet, &memProfile, "memprofile")
	stringVar(flagSet, &registryPath, "r", "registry")
	parseFlags(flagSet, args)

	if time.Time(startTime).IsZero() {
		startTime = timestamp(time.Unix(0, 0))
	}
	if duration == 0 {
		duration = time.Duration(math.MaxInt64)
	}

	args = flagSet.Args()
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
		startTime:  time.Time(startTime),
		endTime:    time.Time(startTime).Add(duration),
		sampleRate: sampleRate,
	}

	records.cpu = wzprof.NewCPUProfiler(wzprof.TimeFunc(records.now))
	records.mem = wzprof.NewMemoryProfiler()
	defer func() {
		records.stop()
		writeProfile("cpu", cpuProfile, records.cpuProfile)
		writeProfile("memory", memProfile, records.memProfile)
	}()

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

	hostModule := wasi_snapshot_preview1.NewHostModule(imports.DetectExtensions(compiledModule)...)
	hostModuleInstance := wazergo.MustInstantiate(ctx, runtime, hostModule, wasi_snapshot_preview1.WithWASI(system))
	ctx = wazergo.WithModuleInstance(ctx, hostModuleInstance)

	return exec(ctx, runtime, compiledModule)
}

type recordProfiler struct {
	records stream.Reader[timemachine.Record]

	cpu *wzprof.CPUProfiler
	mem *wzprof.MemoryProfiler

	cpuProfile *pprof.Profile
	memProfile *pprof.Profile

	currentTime int64
	startTime   time.Time
	endTime     time.Time
	started     bool
	stopped     bool
	sampleRate  float64
	symbols     wzprof.Symbolizer
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
		r.currentTime = records[0].Timestamp()
		if !r.started && !records[0].Time().Before(r.startTime) {
			r.start()
		}
		if !r.stopped && !records[0].Time().Before(r.endTime) {
			r.stop()
		}
	}
	return n, err
}

func (r *recordProfiler) now() int64 {
	return r.currentTime
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
	}
}

func writeProfile(profileName, path string, prof *pprof.Profile) {
	if prof == nil {
		return
	}

	prof.Mapping = []*pprof.Mapping{{
		ID:   1,
		File: "module.wasm",
	}}

	fmt.Printf("writing %s profile:\t%s\n", profileName, path)

	if err := wzprof.WriteProfile(path, prof); err != nil {
		fmt.Printf("ERR: %s\n", err)
	}
}
