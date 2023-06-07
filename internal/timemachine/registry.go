package timemachine

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strconv"
	"sync"
	"time"

	"github.com/google/pprof/profile"
	"github.com/google/uuid"
	"github.com/stealthrocket/timecraft/format"
	"github.com/stealthrocket/timecraft/internal/object"
	"github.com/stealthrocket/timecraft/internal/stream"
	"golang.org/x/exp/slices"
)

// ErrNoRecords is an error returned when no log records could be found for
// a given process id.
var ErrNoLogRecords = errors.New("process has no records")

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

type Registry struct {
	// The object store that the registry uses to load and store data.
	Store object.Store
	// List of tags that are added to every object created by this registry.
	CreateTags []object.Tag
	// List of tags that are added to every query selecting objects from this
	// registry.
	SelectTags []object.Tag
}

func (reg *Registry) CreateModule(ctx context.Context, module *format.Module, tags ...object.Tag) (*format.Descriptor, error) {
	return reg.createObject(ctx, module, tags)
}

func (reg *Registry) CreateRuntime(ctx context.Context, runtime *format.Runtime, tags ...object.Tag) (*format.Descriptor, error) {
	return reg.createObject(ctx, runtime, tags)
}

func (reg *Registry) CreateConfig(ctx context.Context, config *format.Config, tags ...object.Tag) (*format.Descriptor, error) {
	return reg.createObject(ctx, config, tags)
}

func (reg *Registry) CreateProcess(ctx context.Context, process *format.Process, tags ...object.Tag) (*format.Descriptor, error) {
	return reg.createObject(ctx, process, tags)
}

func (reg *Registry) CreateProfile(ctx context.Context, processID format.UUID, profileType string, prof *profile.Profile) (*format.Descriptor, error) {
	buffer := new(bytes.Buffer)
	if err := prof.Write(buffer); err != nil {
		return nil, err
	}

	timeRange := TimeRange{
		Start: time.Unix(0, prof.TimeNanos),
		End:   time.Unix(0, prof.TimeNanos+prof.DurationNanos),
	}

	annotations := make(map[string]string, 2+len(reg.CreateTags))
	assignTags(annotations, reg.CreateTags)
	assignTags(annotations, []object.Tag{{
		Name:  "timecraft.object.mediatype",
		Value: format.TypeTimecraftProfile.String(),
	}, {
		Name:  "timecraft.process.id",
		Value: processID.String(),
	}, {
		Name:  "timecraft.profile.type",
		Value: profileType,
	}, {
		Name:  "timecraft.profile.start",
		Value: timeRange.Start.Format(time.RFC3339Nano),
	}, {
		Name:  "timecraft.profile.end",
		Value: timeRange.End.Format(time.RFC3339Nano),
	}})

	tags := makeTags(annotations)
	hash := HashProfile(processID, profileType, timeRange)
	name := reg.objectKey(hash)
	desc := &format.Descriptor{
		MediaType:   format.TypeTimecraftProfile,
		Digest:      hash,
		Size:        int64(buffer.Len()),
		Annotations: annotations,
	}
	if err := reg.Store.CreateObject(ctx, name, buffer, tags...); err != nil {
		return nil, err
	}
	return desc, nil
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

func (reg *Registry) LookupRecord(ctx context.Context, process *format.Manifest, offset int64) (Record, error) {
	// TODO: handle multiple log segments
	logSegment, err := reg.ReadLogSegment(ctx, process.ProcessID, 0)
	if err != nil {
		return Record{}, err
	}
	defer logSegment.Close()
	segmentReader := NewLogReader(logSegment, process.StartTime)
	defer segmentReader.Close()

	for {
		if ctx.Err() != nil {
			return Record{}, ctx.Err()
		}
		batch, err := segmentReader.ReadRecordBatch()
		if err == io.EOF {
			return Record{}, object.ErrNotExist
		}
		if err != nil {
			return Record{}, ctx.Err()
		}
		it := stream.Iter[Record](batch)
		o := batch.FirstOffset()
		for it.Next() {
			if ctx.Err() != nil {
				return Record{}, ctx.Err()
			}
			if o < offset {
				o++
				continue
			}
			if o > offset {
				return Record{}, fmt.Errorf("requested offset was missed")
			}
			r := it.Value()
			return Record{
				Offset:       o,
				Time:         r.Time,
				FunctionID:   r.FunctionID,
				FunctionCall: append([]byte(nil), r.FunctionCall...),
			}, nil
		}
	}
}

func (reg *Registry) LookupProfile(ctx context.Context, hash format.Hash) (*profile.Profile, error) {
	r, err := reg.Store.ReadObject(ctx, reg.objectKey(hash))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return profile.Parse(r)
}

func (reg *Registry) LookupDescriptor(ctx context.Context, hash format.Hash) (*format.Descriptor, error) {
	return reg.lookupDescriptor(ctx, hash)
}

func (reg *Registry) LookupResource(ctx context.Context, hash format.Hash) (io.ReadSeekCloser, error) {
	return reg.Store.ReadObject(ctx, reg.objectKey(hash))
}

func (reg *Registry) ListModules(ctx context.Context, timeRange TimeRange, tags ...object.Tag) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, format.TypeTimecraftModule, timeRange, tags)
}

func (reg *Registry) ListRuntimes(ctx context.Context, timeRange TimeRange, tags ...object.Tag) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, format.TypeTimecraftRuntime, timeRange, tags)
}

func (reg *Registry) ListConfigs(ctx context.Context, timeRange TimeRange, tags ...object.Tag) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, format.TypeTimecraftConfig, timeRange, tags)
}

func (reg *Registry) ListProcesses(ctx context.Context, timeRange TimeRange, tags ...object.Tag) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, format.TypeTimecraftProcess, timeRange, tags)
}

func (reg *Registry) ListResources(ctx context.Context, mediaType format.MediaType, timeRange TimeRange, tags ...object.Tag) stream.ReadCloser[*format.Descriptor] {
	return reg.listObjects(ctx, mediaType, timeRange, tags)
}

func errorCreateObject(hash format.Hash, value format.Resource, err error) error {
	return fmt.Errorf("create object: %s: %s: %w", hash, value.ContentType(), err)
}

func errorLookupObject(hash format.Hash, value format.Resource, err error) error {
	return fmt.Errorf("lookup object: %s: %s: %w", hash, value.ContentType(), err)
}

func errorLookupDescriptor(hash format.Hash, err error) error {
	return fmt.Errorf("lookup descriptor: %s: %w", hash, err)
}

func appendTagFilters(filters []object.Filter, tags []object.Tag) []object.Filter {
	for _, tag := range tags {
		filters = append(filters, object.MATCH(tag.Name, tag.Value))
	}
	return filters
}

func assignTags(annotations map[string]string, tags []object.Tag) {
	for _, tag := range tags {
		annotations[tag.Name] = tag.Value
	}
}

func makeTags(annotations map[string]string) []object.Tag {
	tags := make([]object.Tag, 0, len(annotations))
	for name, value := range annotations {
		tags = append(tags, object.Tag{
			Name:  name,
			Value: value,
		})
	}
	sortTags(tags)
	return tags
}

func sortTags(tags []object.Tag) {
	slices.SortFunc(tags, func(t1, t2 object.Tag) bool {
		return t1.Name < t2.Name
	})
}

func hashTags(data []byte, tags []object.Tag) format.Hash {
	buf := object.AppendTags(make([]byte, 0, 256), tags...)
	sha := sha256.New()
	sha.Write(data)
	sha.Write(buf)
	return format.Hash{
		Algorithm: "sha256",
		Digest:    hex.EncodeToString(sha.Sum(nil)),
	}
}

func (reg *Registry) createObject(ctx context.Context, value format.ResourceMarshaler, extraTags []object.Tag) (*format.Descriptor, error) {
	data, err := value.MarshalResource()
	if err != nil {
		return nil, err
	}
	mediaType := value.ContentType()

	annotations := make(map[string]string, 1+len(extraTags)+len(reg.CreateTags))
	assignTags(annotations, reg.CreateTags)
	assignTags(annotations, extraTags)
	assignTags(annotations, []object.Tag{{
		Name:  "timecraft.object.mediatype",
		Value: mediaType.String(),
	}})

	tags := makeTags(annotations)
	hash := hashTags(data, tags)
	name := reg.objectKey(hash)
	desc := &format.Descriptor{
		MediaType:   mediaType,
		Digest:      hash,
		Size:        int64(len(data)),
		Annotations: annotations,
	}

	if _, err := reg.Store.StatObject(ctx, name); err != nil {
		if !errors.Is(err, object.ErrNotExist) {
			return nil, errorCreateObject(hash, value, err)
		}
	} else {
		return desc, nil
	}

	if err := reg.Store.CreateObject(ctx, name, bytes.NewReader(data), tags...); err != nil {
		return nil, errorCreateObject(hash, value, err)
	}
	return desc, nil
}

func (reg *Registry) lookupObject(ctx context.Context, hash format.Hash, value format.ResourceUnmarshaler) error {
	if hash.Algorithm != "sha256" || len(hash.Digest) != 64 {
		return errorLookupObject(hash, value, object.ErrNotExist)
	}

	r, err := reg.Store.ReadObject(ctx, reg.objectKey(hash))
	if err != nil {
		return errorLookupObject(hash, value, err)
	}
	defer r.Close()

	var b []byte
	switch f := r.(type) {
	case *os.File:
		var s fs.FileInfo
		s, err = f.Stat()
		if err != nil {
			return errorLookupObject(hash, value, err)
		}
		b = make([]byte, s.Size())
		_, err = io.ReadFull(f, b)
	default:
		b, err = io.ReadAll(r)
	}
	if err != nil {
		return errorLookupObject(hash, value, err)
	}
	if err := value.UnmarshalResource(b); err != nil {
		return errorLookupObject(hash, value, err)
	}
	return nil
}

func (reg *Registry) lookupDescriptor(ctx context.Context, hash format.Hash) (*format.Descriptor, error) {
	if hash.Algorithm != "sha256" {
		return nil, errorLookupDescriptor(hash, object.ErrNotExist)
	}
	if len(hash.Digest) > 64 {
		return nil, errorLookupDescriptor(hash, object.ErrNotExist)
	}
	if len(hash.Digest) < 64 {
		key := reg.objectKey(hash)

		r := reg.Store.ListObjects(ctx, key)
		defer r.Close()

		i := stream.Iter[object.Info](r)
		n := 0
		for i.Next() {
			if n++; n > 1 {
				return nil, errorLookupDescriptor(hash, errors.New("too many objects match the key prefix"))
			}
			key = i.Value().Name
		}
		if err := i.Err(); err != nil {
			return nil, err
		}
		hash = format.ParseHash(path.Base(key))
	}
	info, err := reg.Store.StatObject(ctx, reg.objectKey(hash))
	if err != nil {
		return nil, errorLookupDescriptor(hash, err)
	}
	return newDescriptor(info), nil
}

func newDescriptor(info object.Info) *format.Descriptor {
	annotations := make(map[string]string, len(info.Tags))
	assignTags(annotations, info.Tags)

	mediaType := annotations["timecraft.object.mediatype"]
	delete(annotations, "timecraft.object.mediatype")

	return &format.Descriptor{
		MediaType:   format.MediaType(mediaType),
		Digest:      format.ParseHash(path.Base(info.Name)),
		Size:        info.Size,
		Annotations: annotations,
	}
}

func (reg *Registry) listObjects(ctx context.Context, mediaType format.MediaType, timeRange TimeRange, matchTags []object.Tag) stream.ReadCloser[*format.Descriptor] {
	if !timeRange.Start.IsZero() {
		timeRange.Start = timeRange.Start.Add(-1)
	}

	filters := []object.Filter{
		object.MATCH("timecraft.object.mediatype", mediaType.String()),
		object.AFTER(timeRange.Start),
		object.BEFORE(timeRange.End),
	}
	filters = appendTagFilters(filters, reg.SelectTags)
	filters = appendTagFilters(filters, matchTags)

	reader := reg.Store.ListObjects(ctx, "obj/", filters...)
	return convert(reader, func(info object.Info) (*format.Descriptor, error) {
		return newDescriptor(info), nil
	})
}

func (reg *Registry) objectKey(hash format.Hash) string {
	return "obj/" + hash.String()
}

func (reg *Registry) CreateLogManifest(ctx context.Context, processID format.UUID, manifest *format.Manifest) error {
	b, err := manifest.MarshalResource()
	if err != nil {
		return err
	}
	return reg.Store.CreateObject(ctx, reg.manifestKey(processID), bytes.NewReader(b))
}

func (reg *Registry) CreateLogSegment(ctx context.Context, processID format.UUID, segmentNumber int) (io.WriteCloser, error) {
	r, w := io.Pipe()
	name := reg.logKey(processID, segmentNumber)
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.CloseWithError(reg.Store.CreateObject(ctx, name, r))
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

func (reg *Registry) ListRecords(ctx context.Context, processID format.UUID, timeRange TimeRange) stream.ReadCloser[Record] {
	manifest, err := reg.LookupLogManifest(ctx, processID)
	if err != nil {
		return stream.ErrCloser[Record](err)
	}

	// TODO: handle multiple log segments
	logSegment, err := reg.ReadLogSegment(ctx, processID, 0)
	if err != nil {
		return stream.ErrCloser[Record](err)
	}
	segmentReader := NewLogReader(logSegment, manifest.StartTime)
	recordReader := NewLogRecordReader(segmentReader)

	return stream.NewReadCloser[Record](recordReader, closerFunc(func() error {
		segmentReader.Close()
		logSegment.Close()
		return nil
	}))
}

func (reg *Registry) ListLogSegments(ctx context.Context, processID format.UUID) stream.ReadCloser[format.LogSegment] {
	reader := reg.Store.ListObjects(ctx, "log/"+processID.String()+"/data/")
	return convert(reader, func(info object.Info) (format.LogSegment, error) {
		number := path.Base(info.Name)
		n, err := strconv.ParseInt(number, 16, 32)
		if err != nil || n < 0 {
			return format.LogSegment{}, fmt.Errorf("invalid log segment entry: %q", info.Name)
		}
		segment := format.LogSegment{
			Number:    int(n),
			Size:      info.Size,
			CreatedAt: info.CreatedAt,
		}
		return segment, nil
	})
}

func (reg *Registry) ListLogManifests(ctx context.Context) stream.ReadCloser[*format.Manifest] {
	ch := make(chan stream.Optional[*format.Manifest])
	ctx, cancel := context.WithCancel(ctx)

	go func(reader stream.ReadCloser[object.Info]) {
		defer close(ch)
		defer reader.Close()

		it := stream.Iter[object.Info](reader)
		wg := sync.WaitGroup{}

		for it.Next() {
			processID, err := uuid.Parse(path.Base(it.Value().Name))
			if err != nil {
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				ch <- stream.Opt(reg.LookupLogManifest(ctx, processID))
			}()
		}

		if err := it.Err(); err != nil {
			ch <- stream.Opt[*format.Manifest](nil, err)
		}

		wg.Wait()
	}(reg.Store.ListObjects(ctx, "log/"))

	return stream.NewReadCloser(stream.ChanReader(ch), closerFunc(func() error {
		cancel()
		for range ch {
		}
		return nil
	}))
}

type closerFunc func() error

func (f closerFunc) Close() error { return f() }

func (reg *Registry) LookupLogManifest(ctx context.Context, processID format.UUID) (*format.Manifest, error) {
	r, err := reg.Store.ReadObject(ctx, reg.manifestKey(processID))
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
	m.ProcessID = processID

	segments := reg.ListLogSegments(ctx, processID)
	defer segments.Close()

	m.Segments, err = stream.ReadAll[format.LogSegment](segments)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(m.Segments, func(s1, s2 format.LogSegment) bool {
		return s1.Number < s2.Number
	})
	return m, nil
}

func (reg *Registry) ReadLogSegment(ctx context.Context, processID format.UUID, segmentNumber int) (io.ReadSeekCloser, error) {
	r, err := reg.Store.ReadObject(ctx, reg.logKey(processID, segmentNumber))
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
	return stream.NewReadCloser(stream.ConvertReader[To, From](base, conv), base)
}
