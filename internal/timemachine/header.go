package timemachine

import (
	"errors"
	"io"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
	"github.com/stealthrocket/timecraft/format/types"
	"github.com/stealthrocket/timecraft/internal/buffer"
)

var (
	errMissingRuntime          = errors.New("missing runtime in log header")
	errMissingProcess          = errors.New("missing process in log header")
	errMissingProcessID        = errors.New("missing process id in log header")
	errMissingProcessImage     = errors.New("missing process image in log header")
	errMissingProcessStartTime = errors.New("missing process start time in log header")
)

// Header is a log header.
type Header struct {
	Runtime     Runtime
	Process     Process
	Segment     uint32
	Compression Compression
}

type Runtime struct {
	Runtime string
	Version string
}

type Process struct {
	ID               Hash
	Image            Hash
	StartTime        time.Time
	Args             []string
	Environ          []string
	ParentProcessID  Hash
	ParentForkOffset int64
}

// NewHeader creates a Header from a buffer.
func NewHeader(b []byte) (*Header, error) {
	var h Header
	var header = logsegment.GetSizePrefixedRootAsLogHeader(b, 0)
	var runtime logsegment.Runtime
	if header.Runtime(&runtime) == nil {
		return nil, errMissingRuntime
	}
	h.Runtime.Runtime = string(runtime.Runtime())
	h.Runtime.Version = string(runtime.Version())

	var hash types.Hash
	var process logsegment.Process
	if header.Process(&process) == nil {
		return nil, errMissingProcess
	}
	if process.Id(&hash) == nil {
		return nil, errMissingProcessID
	}
	h.Process.ID = makeHash(&hash)
	if process.Image(&hash) == nil {
		return nil, errMissingProcessImage
	}
	h.Process.Image = makeHash(&hash)
	if unixStartTime := process.UnixStartTime(); unixStartTime == 0 {
		return nil, errMissingProcessStartTime
	} else {
		h.Process.StartTime = time.Unix(0, unixStartTime)
	}
	h.Process.Args = make([]string, process.ArgumentsLength())
	for i := range h.Process.Args {
		h.Process.Args[i] = string(process.Arguments(i))
	}
	h.Process.Environ = make([]string, process.EnvironmentLength())
	for i := range h.Process.Environ {
		h.Process.Environ[i] = string(process.Environment(i))
	}
	if process.ParentProcessId(&hash) != nil {
		h.Process.ParentProcessID = makeHash(&hash)
		h.Process.ParentForkOffset = process.ParentForkOffset()
	}
	h.Segment = header.Segment()
	h.Compression = header.Compression()
	return &h, nil
}

// HeaderBuilder is a builder for headers.
type HeaderBuilder struct {
	builder     *flatbuffers.Builder
	runtime     Runtime
	process     Process
	segment     uint32
	compression Compression
	offsets     []flatbuffers.UOffsetT
	finished    bool
}

// Reset resets the builder.
func (b *HeaderBuilder) Reset() {
	if b.builder == nil {
		b.builder = flatbuffers.NewBuilder(buffer.DefaultSize)
	} else {
		b.builder.Reset()
	}
	b.runtime = Runtime{}
	b.process = Process{}
	b.segment = 0
	b.compression = Uncompressed
	b.offsets = b.offsets[:0]
	b.finished = false
}

// SetRuntime sets runtime information.
func (b *HeaderBuilder) SetRuntime(runtime Runtime) {
	if b.finished {
		panic("builder must be reset before runtime can be set")
	}
	b.runtime = runtime
}

// SetProcess sets process information.
func (b *HeaderBuilder) SetProcess(process Process) {
	if b.finished {
		panic("builder must be reset before process can be set")
	}
	b.process = process
}

// SetSegment sets the log segment.
func (b *HeaderBuilder) SetSegment(segment uint32) {
	if b.finished {
		panic("builder must be reset before segment can be set")
	}
	b.segment = segment
}

// SetCompression sets the compress.
func (b *HeaderBuilder) SetCompression(compression Compression) {
	if b.finished {
		panic("builder must be reset before compression can be set")
	}
	b.compression = compression
}

// Bytes returns the serialized representation of the header.
func (b *HeaderBuilder) Bytes() []byte {
	if !b.finished {
		b.build()
		b.finished = true
	}
	return b.builder.FinishedBytes()
}

// Write writes the serialized representation of the header
// to the specified writer.
func (b *HeaderBuilder) Write(w io.Writer) (int, error) {
	return w.Write(b.Bytes())
}

func (b *HeaderBuilder) build() {
	if b.builder == nil {
		b.builder = flatbuffers.NewBuilder(buffer.DefaultSize)
	}

	processIDOffset := b.prependHash(b.process.ID)
	processImageOffset := b.prependHash(b.process.Image)
	processArgumentsOffset := b.prependStringVector(b.process.Args)
	processEnvironmentOffset := b.prependStringVector(b.process.Environ)

	var parentProcessIDOffset flatbuffers.UOffsetT
	if b.process.ParentProcessID.Digest != "" {
		parentProcessIDOffset = b.prependHash(b.process.ParentProcessID)
	}

	logsegment.ProcessStart(b.builder)
	logsegment.ProcessAddId(b.builder, processIDOffset)
	logsegment.ProcessAddImage(b.builder, processImageOffset)
	logsegment.ProcessAddUnixStartTime(b.builder, b.process.StartTime.UnixNano())
	logsegment.ProcessAddArguments(b.builder, processArgumentsOffset)
	logsegment.ProcessAddEnvironment(b.builder, processEnvironmentOffset)
	if parentProcessIDOffset != 0 {
		logsegment.ProcessAddParentProcessId(b.builder, parentProcessIDOffset)
		logsegment.ProcessAddParentForkOffset(b.builder, b.process.ParentForkOffset)
	}
	processOffset := logsegment.ProcessEnd(b.builder)

	type function struct {
		module, name            flatbuffers.UOffsetT
		paramCount, resultCount uint32
	}

	runtimeNameOffset := b.builder.CreateString(b.runtime.Runtime)
	runtimeVersionOffset := b.builder.CreateString(b.runtime.Version)
	logsegment.RuntimeStart(b.builder)
	logsegment.RuntimeAddRuntime(b.builder, runtimeNameOffset)
	logsegment.RuntimeAddVersion(b.builder, runtimeVersionOffset)
	runtimeOffset := logsegment.RuntimeEnd(b.builder)

	logsegment.LogHeaderStart(b.builder)
	logsegment.LogHeaderAddRuntime(b.builder, runtimeOffset)
	logsegment.LogHeaderAddProcess(b.builder, processOffset)
	logsegment.LogHeaderAddSegment(b.builder, b.segment)
	logsegment.LogHeaderAddCompression(b.builder, b.compression)
	b.builder.FinishSizePrefixed(logsegment.LogHeaderEnd(b.builder))
}

func (b *HeaderBuilder) prependHash(hash Hash) flatbuffers.UOffsetT {
	return hash.prepend(b.builder)
}

func (b *HeaderBuilder) prependStringVector(values []string) flatbuffers.UOffsetT {
	return b.prependObjectVector(len(values), func(i int) flatbuffers.UOffsetT {
		return b.builder.CreateString(values[i])
	})
}

func (b *HeaderBuilder) prependObjectVector(numElems int, create func(int) flatbuffers.UOffsetT) flatbuffers.UOffsetT {
	b.offsets = b.offsets[:0]
	for i := 0; i < numElems; i++ {
		b.offsets = append(b.offsets, create(i))
	}
	return b.prependOffsetVector(b.offsets)
}

func (b *HeaderBuilder) prependOffsetVector(offsets []flatbuffers.UOffsetT) flatbuffers.UOffsetT {
	b.builder.StartVector(flatbuffers.SizeUOffsetT, len(offsets), flatbuffers.SizeUOffsetT)
	for i := len(offsets) - 1; i >= 0; i-- {
		b.builder.PrependUOffsetT(offsets[i])
	}
	return b.builder.EndVector(len(offsets))
}
