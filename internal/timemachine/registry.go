package timemachine

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strconv"
	"time"

	"golang.org/x/exp/slices"

	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/stream"
)

var (
	// ErrNoRecords is an error returned when no log records could be found for
	// a given process id.
	ErrNoLogRecords = errors.New("process has no records")
)

type TimeRange struct {
	Start, End time.Time
}

func Between(then, now time.Time) TimeRange {
	return TimeRange{Start: then, End: now}
}

func Since(then time.Time) TimeRange {
	return Between(then, time.Now().In(then.Location()))
}

func Until(now time.Time) TimeRange {
	return Between(time.Unix(0, 0).In(now.Location()), now)
}

func (tr TimeRange) Duration() time.Duration {
	return tr.End.Sub(tr.Start)
}

type LogSegment struct {
	Number    int
	Size      int64
	CreatedAt time.Time
}

type Registry struct {
	objects object.Store
	tags    []object.Tag
}

func NewRegistry(objects object.Store, tags ...object.Tag) *Registry {
	return &Registry{
		objects: objects,
		tags:    slices.Clone(tags),
	}
}

func (reg *Registry) CreateModule(ctx context.Context, module *format.Module) (*format.Descriptor, error) {
	return reg.createObject(ctx, module)
}

func (reg *Registry) CreateRuntime(ctx context.Context, runtime *format.Runtime) (*format.Descriptor, error) {
	return reg.createObject(ctx, runtime)
}

func (reg *Registry) CreateConfig(ctx context.Context, config *format.Config) (*format.Descriptor, error) {
	return reg.createObject(ctx, config)
}

func (reg *Registry) CreateProcess(ctx context.Context, process *format.Process) (*format.Descriptor, error) {
	return reg.createObject(ctx, process)
}

func (reg *Registry) LookupModule(ctx context.Context, hash format.Hash) (*format.Module, error) {
	module := new(format.Module)
	return module, reg.lookupObject(ctx, hash, module)
}

func (reg *Registry) LookupRuntime(ctx context.Context, hash format.Hash) (*format.Runtime, error) {
	runtime := new(format.Runtime)
	return runtime, reg.lookupObject(ctx, hash, runtime)
}

func (reg *Registry) LookupConfig(ctx context.Context, hash format.Hash) (*format.Config, error) {
	config := new(format.Config)
	return config, reg.lookupObject(ctx, hash, config)
}

func (reg *Registry) LookupProcess(ctx context.Context, hash format.Hash) (*format.Process, error) {
	process := new(format.Process)
	return process, reg.lookupObject(ctx, hash, process)
}

func (reg *Registry) ListModules(ctx context.Context, timeRange TimeRange) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, format.TypeTimecraftModule, timeRange)
}

func (reg *Registry) ListRuntimes(ctx context.Context, timeRange TimeRange) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, format.TypeTimecraftRuntime, timeRange)
}

func (reg *Registry) ListConfigs(ctx context.Context, timeRange TimeRange) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, format.TypeTimecraftConfig, timeRange)
}

func (reg *Registry) ListProcesses(ctx context.Context, timeRange TimeRange) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, format.TypeTimecraftProcess, timeRange)
}

func errorCreateObject(hash format.Hash, value format.Resource, err error) error {
	return fmt.Errorf("create object: %s: %s: %w", hash, value.ContentType(), err)
}

func errorLookupObject(hash format.Hash, value format.Resource, err error) error {
	return fmt.Errorf("lookup object: %s: %s: %w", hash, value.ContentType(), err)
}

func errorListObjects(mediaType format.MediaType, err error) error {
	return fmt.Errorf("list objects: %s: %w", mediaType, err)
}

func (reg *Registry) createObject(ctx context.Context, value format.ResourceMarshaler) (*format.Descriptor, error) {
	b, err := value.MarshalResource()
	if err != nil {
		return nil, err
	}
	hash := SHA256(b)
	name := reg.objectKey(hash)
	desc := &format.Descriptor{
		MediaType: value.ContentType(),
		Digest:    hash,
		Size:      int64(len(b)),
	}

	if _, err := reg.objects.StatObject(ctx, name); err != nil {
		if !errors.Is(err, object.ErrNotExist) {
			return nil, errorCreateObject(hash, value, err)
		}
	} else {
		return desc, nil
	}

	tags := append(slices.Clip(reg.tags), object.Tag{
		Name:  "Content-Type",
		Value: value.ContentType().String(),
	})

	if err := reg.objects.CreateObject(ctx, name, bytes.NewReader(b), tags...); err != nil {
		return nil, errorCreateObject(hash, value, err)
	}
	return desc, nil
}

func (reg *Registry) lookupObject(ctx context.Context, hash format.Hash, value format.ResourceUnmarshaler) error {
	r, err := reg.objects.ReadObject(ctx, reg.objectKey(hash))
	if err != nil {
		return errorLookupObject(hash, value, err)
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return errorLookupObject(hash, value, err)
	}
	if err := value.UnmarshalResource(b); err != nil {
		return errorLookupObject(hash, value, err)
	}
	return nil
}

func (reg *Registry) listObjects(ctx context.Context, mediaType format.MediaType, timeRange TimeRange) stream.ReadCloser[*format.Descriptor] {
	return convert(
		reg.objects.ListObjects(ctx, "obj/",
			object.MATCH("Content-Type", mediaType.String()),
			object.AFTER(timeRange.Start.Add(-1)),
			object.BEFORE(timeRange.End),
		),
		func(info object.Info) (*format.Descriptor, error) {
			hash, err := format.ParseHash(path.Base(info.Name))
			if err != nil {
				return nil, errorListObjects(mediaType, err)
			}
			desc := &format.Descriptor{
				MediaType: mediaType,
				Digest:    hash,
				Size:      info.Size,
			}
			return desc, nil
		},
	)
}

func (reg *Registry) objectKey(hash format.Hash) string {
	return "obj/" + hash.String()
}

func (reg *Registry) CreateLogManifest(ctx context.Context, processID format.UUID, manifest *format.Manifest) error {
	b, err := manifest.MarshalResource()
	if err != nil {
		return err
	}
	return reg.objects.CreateObject(ctx, reg.manifestKey(processID), bytes.NewReader(b))
}

func (reg *Registry) CreateLogSegment(ctx context.Context, processID format.UUID, segmentNumber int) (io.WriteCloser, error) {
	r, w := io.Pipe()
	name := reg.logKey(processID, segmentNumber)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.CloseWithError(reg.objects.CreateObject(ctx, name, r))
	}()
	return &logSegmentWriter{writer: w, done: done}, nil
}

type logSegmentWriter struct {
	writer io.WriteCloser
	done   <-chan struct{}
}

func (w *logSegmentWriter) Write(b []byte) (int, error) {
	return w.writer.Write(b)
}

func (w *logSegmentWriter) Close() error {
	err := w.writer.Close()
	<-w.done
	return err
}

func (reg *Registry) ListLogSegments(ctx context.Context, processID format.UUID) stream.Reader[LogSegment] {
	return convert(reg.objects.ListObjects(ctx, "log/"+processID.String()+"/data"), func(info object.Info) (LogSegment, error) {
		number := path.Base(info.Name)
		n, err := strconv.ParseInt(number, 16, 32)
		if err != nil || n < 0 {
			return LogSegment{}, fmt.Errorf("invalid log segment entry: %q", info.Name)
		}
		segment := LogSegment{
			Number:    int(n),
			Size:      info.Size,
			CreatedAt: info.CreatedAt,
		}
		return segment, nil
	})
}

func (reg *Registry) LookupLogManifest(ctx context.Context, processID format.UUID) (*format.Manifest, error) {
	r, err := reg.objects.ReadObject(ctx, reg.manifestKey(processID))
	if err != nil {
		if errors.Is(err, object.ErrNotExist) {
			err = fmt.Errorf("%w: %s", ErrNoLogRecords, processID)
		}
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	m := new(format.Manifest)
	if err := m.UnmarshalResource(b); err != nil {
		return nil, err
	}
	return m, nil
}

func (reg *Registry) ReadLogSegment(ctx context.Context, processID format.UUID, segmentNumber int) (io.ReadCloser, error) {
	r, err := reg.objects.ReadObject(ctx, reg.logKey(processID, segmentNumber))
	if err != nil {
		if errors.Is(err, object.ErrNotExist) {
			err = fmt.Errorf("%w: %s", ErrNoLogRecords, processID)
		}
	}
	return r, err
}

func (reg *Registry) logKey(processID format.UUID, segmentNumber int) string {
	return fmt.Sprintf("log/%s/data/%08X", processID, segmentNumber)
}

func (reg *Registry) manifestKey(processID format.UUID) string {
	return fmt.Sprintf("log/%s/manifest.json", processID)
}

func convert[To, From any](base stream.ReadCloser[From], conv func(From) (To, error)) stream.ReadCloser[To] {
	return &convertReadCloser[To, From]{base: base, conv: conv}
}

type convertReadCloser[To, From any] struct {
	base stream.ReadCloser[From]
	from []From
	conv func(From) (To, error)
}

func (r *convertReadCloser[To, From]) Close() error {
	return r.base.Close()
}

func (r *convertReadCloser[To, From]) Read(items []To) (n int, err error) {
	for n < len(items) {
		if i := len(items) - n; cap(r.from) <= i {
			r.from = r.from[:i]
		} else {
			r.from = make([]From, i)
		}

		rn, err := r.base.Read(r.from)

		for _, from := range r.from[:rn] {
			to, err := r.conv(from)
			if err != nil {
				r.base.Close()
				return n, err
			}
			items[n] = to
			n++
		}

		if err != nil {
			return n, err
		}
	}
	return n, nil
}
