package timemachine

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/pkg/format/logsegment"
	"github.com/stealthrocket/timecraft/pkg/format/types"
)

type Hash struct {
	Algorithm string
	Digest    string
}

type Compression uint32

const (
	Uncompressed Compression = Compression(types.CompressionUncompressed)
	Snappy       Compression = Compression(types.CompressionSnappy)
	Zstd         Compression = Compression(types.CompressionZstd)
)

type MemoryAccessType uint32

const (
	MemoryRead  MemoryAccessType = MemoryAccessType(logsegment.MemoryAccessTypeMemoryRead)
	MemoryWrite MemoryAccessType = MemoryAccessType(logsegment.MemoryAccessTypeMemoryWrite)
)

type MemoryAccess struct {
	Memory []byte
	Offset uint32
	Access MemoryAccessType
}

type Runtime struct {
	Runtime   string
	Version   string
	Functions []Function
}

type Function struct {
	Module string
	Name   string
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

type LogHeader struct {
	Runtime     Runtime
	Process     Process
	Segment     uint32
	Compression Compression
}

type Record struct {
	Timestamp    time.Time
	Function     int
	Params       []uint64
	Results      []uint64
	MemoryAccess []MemoryAccess
}

var (
	tl0 = []byte("TL.0")
	tl1 = []byte("TL.1")
	tl2 = []byte("TL.2")
	tl3 = []byte("TL.3")
)

const (
	defaultBufferSize = 16 * 1024
)

type LogReader struct {
	input   *bufio.Reader
	header  LogHeader
	batch   []Record
	frame   []byte
	discard int
	buffer  bytes.Buffer
}

func NewLogReader(input io.Reader) *LogReader {
	return NewLogReaderSize(input, defaultBufferSize)
}

func NewLogReaderSize(input io.Reader, bufferSize int) *LogReader {
	return &LogReader{
		input: bufio.NewReaderSize(input, bufferSize),
	}
}

func (r *LogReader) Reset(input io.Reader) {
	r.input.Reset(input)
	r.header = LogHeader{}
	r.batch = nil
	r.buffer.Reset()
}

func (r *LogReader) ReadLogHeader() (*LogHeader, error) {
	b, err := r.readFrame()
	if err != nil {
		return nil, err
	}

	_ = b

	// header := logsegment.GetSizePrefixedRootAsLogHeader(b, 0)

	// runtime := logsegment.Runtime{}
	// header.Runtime(&runtime)

	// process := logsegment.Process{}
	// header.Process(&process)

	return nil, io.EOF
}

func (r *LogReader) ReadRecordBatch() ([]Record, error) {
	return nil, io.EOF
}

func (r *LogReader) readFrame() ([]byte, error) {
	if r.discard > 0 {
		n, err := r.input.Discard(r.discard)
		r.discard -= n
		if err != nil {
			return nil, err
		}
	}
	b, err := r.input.Peek(4)
	if err != nil {
		return nil, err
	}
	n := 4 + int(binary.LittleEndian.Uint32(b))
	b, err = r.input.Peek(n)
	if err == nil {
		r.discard = n
		return b, nil
	}
	if !errors.Is(err, bufio.ErrBufferFull) {
		return nil, err
	}
	if n <= cap(r.frame) {
		r.frame = r.frame[:n]
	} else {
		r.frame = make([]byte, n)
	}
	_, err = io.ReadFull(r.input, r.frame)
	return r.frame, err
}

// LogWriter supports writing log segments to an io.Writer.
//
// The type has two main methods, WriteLogHeader and WriteRecordBatch.
// The former must be called first and only once to write the log header and
// initialize the state of the writer, zero or more record batches may then
// be written to the log after that.
type LogWriter struct {
	output  io.Writer
	builder *flatbuffers.Builder
	buffer  *bytes.Buffer
	// The start time is captured when writing the log header to compute
	// monontonic timestamps for the records written to the log.
	startTime time.Time
	// This field keeps track of the logical offset for the next record
	// batch that will be written to the log.
	nextOffset int64
	// When writing to the underlying io.Writer causes an error, we stop
	// accepting writes and assume the log is corrupted.
	stickyErr error
	// Those fields are local buffers retained as an optimization to avoid
	// reallocation of temporary arrays when serializing log records.
	records []flatbuffers.UOffsetT
	offsets []flatbuffers.UOffsetT
}

// NewLogWriter constructs a new log writer which produces output to the given
// io.Writer.
func NewLogWriter(output io.Writer) *LogWriter {
	return NewLogWriterSize(output, defaultBufferSize)
}

// NewLogWriterSize is like NewLogWriter but it lets the application configure
// the initial buffer size.
func NewLogWriterSize(output io.Writer, bufferSize int) *LogWriter {
	return &LogWriter{
		output:  output,
		builder: flatbuffers.NewBuilder(bufferSize),
		buffer:  bytes.NewBuffer(make([]byte, 0, bufferSize)),
	}
}

// Reset resets the state of the log writer to produce to output to the given
// io.Writer.
//
// WriteLogHeader should be called again after resetting the writer.
func (w *LogWriter) Reset(output io.Writer) {
	w.output = output
	w.builder.Reset()
	w.buffer.Reset()
	w.startTime = time.Time{}
	w.nextOffset = 0
	w.stickyErr = nil
	w.records = w.records[:0]
	w.offsets = w.offsets[:0]
}

// WriteLogHeader writes the log header. The method must be called before any
// records are written to the log via calls to WriteRecordBatch.
func (w *LogWriter) WriteLogHeader(header *LogHeader) error {
	if w.stickyErr != nil {
		return w.stickyErr
	}
	w.builder.Reset()

	processID := w.prependHash(header.Process.ID)
	processImage := w.prependHash(header.Process.Image)
	processArguments := w.prependStringVector(header.Process.Args)
	processEnvironment := w.prependStringVector(header.Process.Environ)

	var parentProcessID flatbuffers.UOffsetT
	if header.Process.ParentProcessID.Digest != "" {
		parentProcessID = w.prependHash(header.Process.ParentProcessID)
	}

	logsegment.ProcessStart(w.builder)
	logsegment.ProcessAddId(w.builder, processID)
	logsegment.ProcessAddImage(w.builder, processImage)
	logsegment.ProcessAddUnixStartTime(w.builder, header.Process.StartTime.UnixNano())
	logsegment.ProcessAddArguments(w.builder, processArguments)
	logsegment.ProcessAddEnvironment(w.builder, processEnvironment)
	if parentProcessID != 0 {
		logsegment.ProcessAddParentProcessId(w.builder, parentProcessID)
		logsegment.ProcessAddParentForkOffset(w.builder, header.Process.ParentForkOffset)
	}
	processOffset := logsegment.ProcessEnd(w.builder)

	type function struct {
		module, name flatbuffers.UOffsetT
	}

	functionOffsets := make([]function, 0, 64)
	if len(header.Runtime.Functions) <= cap(functionOffsets) {
		functionOffsets = functionOffsets[:len(header.Runtime.Functions)]
	} else {
		functionOffsets = make([]function, len(header.Runtime.Functions))
	}

	for i, fn := range header.Runtime.Functions {
		functionOffsets[i] = function{
			module: w.builder.CreateSharedString(fn.Module),
			name:   w.builder.CreateString(fn.Name),
		}
	}

	functions := w.prependObjectVector(len(header.Runtime.Functions),
		func(i int) flatbuffers.UOffsetT {
			logsegment.FunctionStart(w.builder)
			logsegment.FunctionAddModule(w.builder, functionOffsets[i].module)
			logsegment.FunctionAddName(w.builder, functionOffsets[i].name)
			return logsegment.FunctionEnd(w.builder)
		},
	)

	runtime := w.builder.CreateString(header.Runtime.Runtime)
	version := w.builder.CreateString(header.Runtime.Version)
	logsegment.RuntimeStart(w.builder)
	logsegment.RuntimeAddRuntime(w.builder, runtime)
	logsegment.RuntimeAddVersion(w.builder, version)
	logsegment.RuntimeAddFunctions(w.builder, functions)
	runtimeOffset := logsegment.RuntimeEnd(w.builder)

	logsegment.LogHeaderStart(w.builder)
	logsegment.LogHeaderAddRuntime(w.builder, runtimeOffset)
	logsegment.LogHeaderAddProcess(w.builder, processOffset)
	logsegment.LogHeaderAddSegment(w.builder, header.Segment)
	logsegment.LogHeaderAddCompression(w.builder, types.Compression(header.Compression))
	logHeader := logsegment.LogHeaderEnd(w.builder)

	w.builder.FinishSizePrefixedWithFileIdentifier(logHeader, tl0)

	if _, err := w.output.Write(w.builder.FinishedBytes()); err != nil {
		w.stickyErr = err
		return err
	}
	w.startTime = header.Process.StartTime
	return nil
}

// WriteRecordBatch writes a record batch to the log. The method returns the
// logical offset of the first record, or a non-nil error if the write failed.
//
// If the error occurred while writing to the underlying io.Writer, the writer
// is broken and will always error on future calls to WriteRecordBatch until
// the program calls Reset.
func (w *LogWriter) WriteRecordBatch(batch []Record) (int64, error) {
	if w.stickyErr != nil {
		return w.nextOffset, w.stickyErr
	}
	w.builder.Reset()
	w.buffer.Reset()
	w.records = w.records[:0]

	uncompressedSize := uint32(0)

	for _, record := range batch {
		offset := uncompressedSize
		length := uint32(0)

		for _, access := range record.MemoryAccess {
			uncompressedSize += uint32(len(access.Memory))
			length += uint32(len(access.Memory))

			if _, err := w.buffer.Write(access.Memory); err != nil {
				return w.nextOffset, err
			}
		}

		logsegment.RecordStartMemoryAccessVector(w.builder, len(record.MemoryAccess))
		recordOffset := uncompressedSize

		for i := len(record.MemoryAccess) - 1; i >= 0; i-- {
			access := &record.MemoryAccess[i]
			recordOffset -= uint32(len(access.Memory))
			w.prependMemoryAccess(recordOffset, access)
		}

		memory := w.builder.EndVector(len(record.MemoryAccess))
		params := w.prependUint64Vector(record.Params)
		results := w.prependUint64Vector(record.Results)
		timestamp := int64(record.Timestamp.Sub(w.startTime))
		function := uint32(record.Function)

		logsegment.RecordStart(w.builder)
		logsegment.RecordAddTimestamp(w.builder, timestamp)
		logsegment.RecordAddFunction(w.builder, function)
		logsegment.RecordAddParams(w.builder, params)
		logsegment.RecordAddResults(w.builder, results)
		logsegment.RecordAddOffset(w.builder, offset)
		logsegment.RecordAddLength(w.builder, length)
		logsegment.RecordAddMemoryAccess(w.builder, memory)
		w.records = append(w.records, logsegment.RecordEnd(w.builder))
	}

	firstOffset := w.nextOffset
	w.nextOffset += int64(len(batch))

	records := w.prependOffsetVector(w.records)
	compressedSize := uint32(w.buffer.Len())

	logsegment.RecordBatchStart(w.builder)
	logsegment.RecordBatchAddFirstOffset(w.builder, firstOffset)
	logsegment.RecordBatchAddCompressedSize(w.builder, compressedSize)
	logsegment.RecordBatchAddUncompressedSize(w.builder, uncompressedSize)
	logsegment.RecordBatchAddChecksum(w.builder, 0)
	logsegment.RecordBatchAddRecords(w.builder, records)
	w.builder.FinishSizePrefixed(logsegment.RecordBatchEnd(w.builder))

	if _, err := w.output.Write(w.builder.FinishedBytes()); err != nil {
		w.stickyErr = err
		return firstOffset, err
	}
	if _, err := w.output.Write(w.buffer.Bytes()); err != nil {
		w.stickyErr = err
		return firstOffset, err
	}
	return firstOffset, nil
}

func (w *LogWriter) prependHash(hash Hash) flatbuffers.UOffsetT {
	algorithm := w.builder.CreateSharedString(hash.Algorithm)
	digest := w.builder.CreateString(hash.Digest)
	types.HashStart(w.builder)
	types.HashAddAlgorithm(w.builder, algorithm)
	types.HashAddDigest(w.builder, digest)
	return types.HashEnd(w.builder)
}

func (w *LogWriter) prependStringVector(values []string) flatbuffers.UOffsetT {
	return w.prependObjectVector(len(values), func(i int) flatbuffers.UOffsetT {
		return w.builder.CreateString(values[i])
	})
}

func (w *LogWriter) prependUint64Vector(values []uint64) flatbuffers.UOffsetT {
	w.builder.StartVector(8, len(values), 8)
	for i := len(values) - 1; i >= 0; i-- {
		w.builder.PlaceUint64(values[i])
	}
	return w.builder.EndVector(len(values))
}

func (w *LogWriter) prependObjectVector(numElems int, create func(int) flatbuffers.UOffsetT) flatbuffers.UOffsetT {
	if numElems <= cap(w.offsets) {
		w.offsets = w.offsets[:numElems]
	} else {
		w.offsets = make([]flatbuffers.UOffsetT, numElems)
	}
	for i := range w.offsets {
		w.offsets[i] = create(i)
	}
	return w.prependOffsetVector(w.offsets)
}

func (w *LogWriter) prependOffsetVector(offsets []flatbuffers.UOffsetT) flatbuffers.UOffsetT {
	w.builder.StartVector(4, len(offsets), 4)
	for i := len(offsets) - 1; i >= 0; i-- {
		w.builder.PlaceUOffsetT(offsets[i])
	}
	return w.builder.EndVector(len(offsets))
}

// prependMemoryAccess is likst the generated logsegment.CreateMemoryAccess but
// it uses PlaceUint32 instead of PrependUint32 for higher efficiency.
//
// Using this custom function is useful because the memory access are in the
// inner-most loop of the writer and the most common type of values written.
func (w *LogWriter) prependMemoryAccess(offset uint32, access *MemoryAccess) flatbuffers.UOffsetT {
	w.builder.Prep(4, 16)
	w.builder.PlaceUint32(uint32(access.Access))
	w.builder.PlaceUint32(uint32(len(access.Memory)))
	w.builder.PlaceUint32(offset)
	w.builder.PlaceUint32(access.Offset)
	return w.builder.Offset()
}