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

	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/stream"
)

var (
	// ErrNoRecords is an error returned when no log records could be found for
	// a given process id.
	ErrNoLogRecords = errors.New("process has no records")
)

type ModuleInfo struct {
	ID        Hash
	Size      int64
	CreatedAt time.Time
}

type LogSegmentInfo struct {
	Number    int
	Size      int64
	CreatedAt time.Time
}

type Registry struct {
	objects object.Store
}

func NewRegistry(objects object.Store) *Registry {
	return &Registry{objects: objects}
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

func (reg *Registry) LookupDescriptor(ctx context.Context, hash format.Hash) (*format.Descriptor, error) {
	return reg.lookupDescriptor(ctx, reg.descriptorKey(hash))
}

func errorCreateObject(hash format.Hash, value format.Resource, err error) error {
	return fmt.Errorf("create object: %s: %s: %w", hash, value.ContentType(), err)
}

func errorLookupObject(hash format.Hash, value format.Resource, err error) error {
	return fmt.Errorf("lookup object: %s: %s: %w", hash, value.ContentType(), err)
}

func errorLookupDescriptor(hash format.Hash, value format.Resource, err error) error {
	return fmt.Errorf("lookup descriptor: %s: %s: %w", hash, value.ContentType(), err)
}

func (reg *Registry) createObject(ctx context.Context, value format.ResourceMarshaler) (*format.Descriptor, error) {
	b, err := value.MarshalResource()
	if err != nil {
		return nil, err
	}
	hash := SHA256(b)
	name := reg.objectKey(hash)
	desc := reg.descriptorKey(hash)

	descriptor, err := reg.lookupDescriptor(ctx, desc)
	if err == nil {
		return descriptor, nil
	}
	if !errors.Is(err, object.ErrNotExist) {
		return nil, errorLookupDescriptor(hash, value, err)
	}

	descriptor = &format.Descriptor{
		MediaType: value.ContentType(),
		Digest:    hash,
		Size:      int64(len(b)),
	}
	d, err := descriptor.MarshalResource()
	if err != nil {
		return nil, errorCreateObject(hash, value, err)
	}

	if err := reg.objects.CreateObject(ctx, desc, bytes.NewReader(d)); err != nil {
		return nil, errorCreateObject(hash, value, err)
	}
	if err := reg.objects.CreateObject(ctx, name, bytes.NewReader(b)); err != nil {
		return nil, errorCreateObject(hash, value, err)
	}
	return descriptor, nil
}

func (reg *Registry) lookupDescriptor(ctx context.Context, key string) (*format.Descriptor, error) {
	r, err := reg.objects.ReadObject(ctx, key)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	descriptor := new(format.Descriptor)
	if err := descriptor.UnmarshalResource(b); err != nil {
		return nil, err
	}
	return descriptor, nil
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

func (reg *Registry) descriptorKey(hash format.Hash) string {
	return "obj/" + hash.String() + "/descriptor.json"
}

func (reg *Registry) objectKey(hash format.Hash) string {
	return "obj/" + hash.String() + "/content"
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

func (reg *Registry) ListLogSegments(ctx context.Context, processID format.UUID) stream.Reader[LogSegmentInfo] {
	return convert(reg.objects.ListObjects(ctx, "log/"+processID.String()+"/data"), func(info object.Info) (LogSegmentInfo, error) {
		number := path.Base(info.Name)
		n, err := strconv.ParseInt(number, 16, 32)
		if err != nil || n < 0 {
			return LogSegmentInfo{}, fmt.Errorf("invalid log segment entry: %q", info.Name)
		}
		segment := LogSegmentInfo{
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
