package timecraft

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
	"github.com/stealthrocket/wasi-go/imports"
	"github.com/stealthrocket/wasi-go/imports/wasi_snapshot_preview1"
	"github.com/stealthrocket/wazergo"
	"github.com/tetratelabs/wazero"
)

// Replay coordinates the replay of WebAssembly modules.
type Replay struct {
	registry  *timemachine.Registry
	runtime   wazero.Runtime
	processID uuid.UUID

	stdout io.Writer
	stderr io.Writer
	trace  io.Writer
}

// NewReplay creates a Replay for a WebAssembly modules with a recorded trace
// of execution, identified by processID.
func NewReplay(registry *timemachine.Registry, runtime wazero.Runtime, processID uuid.UUID) *Replay {
	return &Replay{
		registry:  registry,
		runtime:   runtime,
		processID: processID,
	}
}

// SetStdout sets the io.Writer that receives stdout from the replay.
func (r *Replay) SetStdout(w io.Writer) {
	r.stdout = w
}

// SetStderr sets the io.Writer that receives stderr from the replay.
func (r *Replay) SetStderr(w io.Writer) {
	r.stderr = w
}

// SetTrace sets the io.Writer that receives a trace of system calls from the
// replay.
func (r *Replay) SetTrace(w io.Writer) {
	r.trace = w
}

// Replay replays process execution.
func (r *Replay) Replay(ctx context.Context) error {
	moduleCode, function, err := r.ModuleCode(ctx)
	if err != nil {
		return err
	}

	records, _, err := r.RecordReader(ctx)
	if err != nil {
		return err
	}
	defer records.Close()

	return r.ReplayRecords(ctx, function, moduleCode, records)
}

// ModuleCode reads the module's WebAssembly code.
func (r *Replay) ModuleCode(ctx context.Context) ([]byte, string, error) {
	manifest, err := r.registry.LookupLogManifest(ctx, r.processID)
	if err != nil {
		return nil, "", err
	}
	fn := lookupFunction(manifest)
	process, err := r.registry.LookupProcess(ctx, manifest.Process.Digest)
	if err != nil {
		return nil, "", err
	}
	processConfig, err := r.registry.LookupConfig(ctx, process.Config.Digest)
	if err != nil {
		return nil, "", err
	}
	module, err := r.registry.LookupModule(ctx, processConfig.Modules[0].Digest)
	if err != nil {
		return nil, "", err
	}
	return module.Code, fn, nil
}

func lookupFunction(m *format.Manifest) string {
	for key, value := range m.Process.Annotations {
		// TODO: create well known tags list
		if key == "timecraft.module.function" {
			return value
		}
	}
	return ""
}

// RecordReader constructs a reader for the process replay log.
func (r *Replay) RecordReader(ctx context.Context) (records stream.ReadCloser[timemachine.Record], startTime time.Time, err error) {
	manifest, err := r.registry.LookupLogManifest(ctx, r.processID)
	if err != nil {
		return nil, time.Time{}, err
	}
	// TODO: return a reader that reads from many segments
	logSegment, err := r.registry.ReadLogSegment(ctx, r.processID, 0)
	if err != nil {
		return nil, time.Time{}, err
	}
	logReader := timemachine.NewLogReader(logSegment, manifest)
	recordReader := timemachine.NewLogRecordReader(logReader)
	return &recordReadCloser{recordReader, logReader, logSegment}, manifest.StartTime, nil
}

// ReplayRecordsModule replays process execution using the specified records on
// a pre-compiled module.
func (r *Replay) ReplayRecordsModule(ctx context.Context, function string, compiledModule wazero.CompiledModule, records stream.Reader[timemachine.Record]) error {
	replay := wasicall.NewReplay(records)
	defer replay.Close(ctx)

	system := wasicall.NewFallbackSystem(replay, wasicall.NewExitSystem(0))

	if r.stdout != nil {
		replay.Stdout = r.stdout
	}
	if r.stdout != nil {
		replay.Stderr = r.stderr
	}
	if r.trace != nil {
		system = wasi.Trace(r.trace, system)
	}

	hostModule := wasi_snapshot_preview1.NewHostModule(imports.DetectExtensions(compiledModule)...)
	hostModuleInstance := wazergo.MustInstantiate(ctx, r.runtime, hostModule, wasi_snapshot_preview1.WithWASI(system))
	ctx = wazergo.WithModuleInstance(ctx, hostModuleInstance)

	return runModule(ctx, r.runtime, compiledModule, function)
}

// ReplayRecords replays process execution using the specified records.
func (r *Replay) ReplayRecords(ctx context.Context, function string, moduleCode []byte, records stream.Reader[timemachine.Record]) error {
	compiledModule, err := r.runtime.CompileModule(ctx, moduleCode)
	if err != nil {
		return err
	}
	defer compiledModule.Close(ctx)
	return r.ReplayRecordsModule(ctx, function, compiledModule, records)
}

type recordReadCloser struct {
	stream.Reader[timemachine.Record]

	logReader  io.Closer
	logSegment io.Closer
}

func (r *recordReadCloser) Close() error {
	if err := r.logReader.Close(); err != nil {
		r.logSegment.Close()
		return err
	}
	return r.logSegment.Close()
}
