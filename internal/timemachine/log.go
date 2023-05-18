package timemachine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/klauspost/compress/snappy"
	"github.com/klauspost/compress/zstd"
	"github.com/stealthrocket/timecraft/format/logsegment"
	"github.com/stealthrocket/timecraft/format/types"
)

type Compression = types.Compression

const (
	Uncompressed Compression = types.CompressionUncompressed
	Snappy       Compression = types.CompressionSnappy
	Zstd         Compression = types.CompressionZstd
)

type MemoryAccess struct {
	Memory []byte
	Offset uint32
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

	// Stack is buffer space for params/results.
	Stack [10]uint64
}

var (
	errMissingRuntime          = errors.New("missing runtime in log header")
	errMissingProcess          = errors.New("missing process in log header")
	errMissingProcessID        = errors.New("missing process id in log header")
	errMissingProcessImage     = errors.New("missing process image in log header")
	errMissingProcessStartTime = errors.New("missing process start time in log header")
)

const (
	defaultBufferSize = 4096
	maxFrameSize      = (1 * 1024 * 1024) - 4
)

// RecordBatch represents a single set of records read from a log segment.
type RecordBatch struct {
	// Original input and location of the record batch in it. The byte offset
	// is where the record batch started, the byte length is its size until the
	// beginning of the data section.
	input      io.ReaderAt
	byteOffset int64
	byteLength int64

	// Flatbuffers pointer into the record batch frame used to load the records.
	batch logsegment.RecordBatch

	// Capture of the log header for the segment that the record batch was read
	// from.
	header *LogHeader

	// The record batch keeps ownership of the frame that it was read from;
	// the frame is released to the global pool when the batch is closed.
	frame  *buffer
	memory *buffer
	// Loading of the memory buffer is synchronized on this once value so it may
	// happen when records are read concurrently.
	//
	// If an error occurs while reading, it is captured in `err` and all reads of
	// the memory regions will observe the error.
	once sync.Once
	err  error
}

// Close must be called by the application when the batch isn't needed anymore
// to allow the resources held internally to be reused by the application.
func (r *RecordBatch) Close() error {
	r.once.Do(func() {})
	memoryBufferPool := &compressedBufferPool
	if r.header.Compression == Uncompressed {
		memoryBufferPool = &uncompressedBufferPool
	}
	releaseBuffer(&r.memory, memoryBufferPool)
	releaseBuffer(&r.frame, &frameBufferPool)
	r.batch = logsegment.RecordBatch{}
	return nil
}

// Compression returns the compression algorithm used to encode the record
// batch data section.
func (r *RecordBatch) Compression() Compression {
	return r.header.Compression
}

// FirstOffset returns the logical offset of the first record in the batch.
func (r *RecordBatch) FirstOffset() int64 {
	return r.batch.FirstOffset()
}

// CompressedSize returns the size of the record batch data section in the log
// segment.
func (r *RecordBatch) CompressedSize() int64 {
	return int64(r.batch.CompressedSize())
}

// UncompressedSize returns the size of the record batch data section after
// being uncompressed.
func (r *RecordBatch) UncompressedSize() int64 {
	return int64(r.batch.UncompressedSize())
}

// NumRecords returns the number of records in the batch.
func (r *RecordBatch) NumRecords() int {
	return int(r.batch.RecordsLength())
}

// Records is a helper function which reads all records of a batch in memory.
//
// The method is useful in contexts with relaxed performance constraints, as the
// returned values are heap allocated and hold copies of the underlying memory
// buffers.
func (r *RecordBatch) Records() []Record {
	records := make([]Record, r.NumRecords())
	for i := range records {
		rr := r.RecordReaderAt(i)
		records[i] = Record{
			Timestamp:    rr.Timestamp(),
			Function:     rr.Function(),
			Params:       rr.Params(),
			Results:      rr.Results(),
			MemoryAccess: rr.MemoryAccess(),
		}
	}
	return records
}

// RecordReaderAt returns a reader positioned on the record at the given index
// in the batch.
//
// The index is local to the batch, to translate a logical record offset into
// an index, subtract the value of the first record offset of the batch.
//
// The returned record reader remaims valid to use until the batch is closed.
func (r *RecordBatch) RecordReaderAt(i int) RecordReader {
	rr := RecordReader{batch: r}
	r.batch.Records(&rr.record, i)
	return rr
}

func (r *RecordBatch) loadMemory() ([]byte, error) {
	r.once.Do(func() {
		r.memory, r.err = r.readMemory()
	})
	if r.err != nil {
		return nil, r.err
	}
	return r.memory.data, nil
}

func (r *RecordBatch) readMemory() (*buffer, error) {
	compression := r.header.Compression
	compressedSize := int(r.CompressedSize())
	uncompressedSize := int(r.UncompressedSize())

	var memoryBufferPool *bufferPool
	var memoryBufferSize int
	if compression == Uncompressed {
		memoryBufferPool = &uncompressedBufferPool
		memoryBufferSize = uncompressedSize
	} else {
		memoryBufferPool = &compressedBufferPool
		memoryBufferSize = compressedSize
	}

	memoryBuffer := memoryBufferPool.get(memoryBufferSize)

	_, err := r.input.ReadAt(memoryBuffer.data, r.byteOffset+r.byteLength)
	if err != nil {
		memoryBufferPool.put(memoryBuffer)
		return nil, err
	}

	if compression == Uncompressed {
		return memoryBuffer, nil
	}
	defer memoryBufferPool.put(memoryBuffer)

	uncompressedMemoryBuffer := uncompressedBufferPool.get(uncompressedSize)
	src := memoryBuffer.data
	dst := uncompressedMemoryBuffer.data
	dst, err = decompress(dst, src, compression)
	uncompressedMemoryBuffer.data = dst
	return uncompressedMemoryBuffer, err
}

// RecordReader values are returned by calling RecordReaderAt on a RecordBatch,
// which lets the program read a single record from a batch.
type RecordReader struct {
	batch  *RecordBatch
	record logsegment.Record
}

// Timestamp returns the time at which the record was produced.
func (r *RecordReader) Timestamp() time.Time {
	return r.batch.header.Process.StartTime.Add(time.Duration(r.record.Timestamp()))
}

// Function returns the index of the function that produced the record.
func (r *RecordReader) Function() int {
	return int(r.record.Function())
}

// LookupFunction returns a Function object representing the function that
// produced the record.
func (r *RecordReader) LookupFunction() (Function, bool) {
	if i := r.Function(); i >= 0 && i < len(r.batch.header.Runtime.Functions) {
		return r.batch.header.Runtime.Functions[i], true
	}
	return Function{}, false
}

// NumParams returns the number of parameters that were passed to the function.
func (r *RecordReader) NumParams() int {
	return int(r.record.ParamsLength())
}

// ReadParams reads the function parameters into the slice passed as argument.
func (r *RecordReader) ReadParams(params []uint64) {
	for i := range params {
		params[i] = r.record.Params(i)
	}
}

// ParamAt returns the param at the specified index.
func (r *RecordReader) ParamAt(i int) uint64 {
	return r.record.Params(i)
}

// Params returns the function parameters as a newly allocated slice.
func (r *RecordReader) Params() []uint64 {
	numParams := r.NumParams()
	if numParams == 0 {
		return nil
	}
	stack := make([]uint64, numParams)
	r.ReadParams(stack)
	return stack
}

// NumResults returns the number of results that were returned by the function.
func (r *RecordReader) NumResults() int {
	return int(r.record.ResultsLength())
}

// ReadResults reads the function results into the slice passed as argument.
func (r *RecordReader) ReadResults(results []uint64) {
	for i := range results {
		results[i] = r.record.Results(i)
	}
}

// ResultAt returns the param at the specified index.
func (r *RecordReader) ResultAt(i int) uint64 {
	return r.record.Results(i)
}

// Results returns the function results as a newly allocated slice.
func (r *RecordReader) Results() []uint64 {
	numResults := r.NumResults()
	if numResults == 0 {
		return nil
	}
	stack := make([]uint64, numResults)
	r.ReadResults(stack)
	return stack
}

// NumMemoryAccess returns the number of memory access recorded in r.
func (r *RecordReader) NumMemoryAccess() int {
	return int(r.record.MemoryAccessLength())
}

// ReadMemoryAccess reads memory access for r int the slice passed as argument.
//
// Byte slices written to the slice elements' Memory field remain valid until
// the parent record batch is closed.
func (r *RecordReader) ReadMemoryAccess(memoryAccess []MemoryAccess) {
	for i := range memoryAccess {
		memoryAccess[i] = r.MemoryAccessAt(i)
	}
	return
}

// MemoryAccessAt returns the memory access at the specified index.
func (r *RecordReader) MemoryAccessAt(i int) MemoryAccess {
	m := logsegment.MemoryAccess{}
	r.record.MemoryAccess(&m, i)
	return MemoryAccess{
		Memory: m.MemoryBytes(),
		Offset: m.Offset(),
	}
}

// MemoryAccess reads and returns the memory access recorded by r.
func (r *RecordReader) MemoryAccess() []MemoryAccess {
	numMemoryAccess := r.NumMemoryAccess()
	if numMemoryAccess == 0 {
		return nil
	}
	memoryAccess := make([]MemoryAccess, numMemoryAccess)
	r.ReadMemoryAccess(memoryAccess)
	return memoryAccess
}

// LogReader instances allow programs to read the content of a record log.
//
// The LogReader type has two main methods, ReadLogHeader and ReadRecordBatch.
// ReadLogHeader should be called first to load the header needed to read
// log records. ReadRecrdBatch may be called multiple times until io.EOF is
// returned to scan through the log.
//
// Because the log reader is based on a io.ReaderAt, and positioning is done
// by the application by passing the byte offset where the record batch should
// be read from, it is safe to perform concurrent reads from the log.
type LogReader struct {
	input      io.ReaderAt
	bufferSize int
}

// NewLogReader construct a new log reader consuming input from the given
// io.Reader.
func NewLogReader(input io.ReaderAt) *LogReader {
	return NewLogReaderSize(input, defaultBufferSize)
}

// NewLogReaderSize is like NewLogReader but it allows the program to configure
// the read buffer size.
func NewLogReaderSize(input io.ReaderAt, bufferSize int) *LogReader {
	return &LogReader{
		input:      input,
		bufferSize: align(bufferSize),
	}
}

// Reset clears the reader state and sets it to consume input from the given
// io.Reader.
func (r *LogReader) Reset(input io.ReaderAt) {
	r.input = input
}

// ReadLogHeader reads and returns the log header from r.
//
// The log header is always located at the first byte of the underlying segment.
//
// The method returns the log header that was read, along with the number of
// bytes that it spanned over. If the log header could not be read, a non-nil
// error is returned describing the reason why.
func (r *LogReader) ReadLogHeader() (*LogHeader, int64, error) {
	f, err := r.readFrameAt(0)
	if err != nil {
		return nil, 0, err
	}
	defer frameBufferPool.put(f)

	var h LogHeader
	var header = logsegment.GetRootAsLogHeader(f.data[4:], 0)
	var runtime logsegment.Runtime
	if header.Runtime(&runtime) == nil {
		return nil, 0, errMissingRuntime
	}
	h.Runtime.Runtime = string(runtime.Runtime())
	h.Runtime.Version = string(runtime.Version())
	h.Runtime.Functions = make([]Function, runtime.FunctionsLength())

	for i := range h.Runtime.Functions {
		f := logsegment.Function{}
		if !runtime.Functions(&f, i) {
			return nil, 0, fmt.Errorf("missing runtime function in log header: expected %d but could not load function at index %d", len(h.Runtime.Functions), i)
		}
		h.Runtime.Functions[i] = Function{
			Module: string(f.Module()),
			Name:   string(f.Name()),
		}
	}

	var hash types.Hash
	var process logsegment.Process
	if header.Process(&process) == nil {
		return nil, 0, errMissingProcess
	}
	if process.Id(&hash) == nil {
		return nil, 0, errMissingProcessID
	}
	h.Process.ID = makeHash(&hash)
	if process.Image(&hash) == nil {
		return nil, 0, errMissingProcessImage
	}
	h.Process.Image = makeHash(&hash)
	if unixStartTime := process.UnixStartTime(); unixStartTime == 0 {
		return nil, 0, errMissingProcessStartTime
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
	return &h, int64(len(f.data)), nil
}

func (r *LogReader) ReadRecordBatch(header *LogHeader, byteOffset int64) (*RecordBatch, int64, error) {
	f, err := r.readFrameAt(byteOffset)
	if err != nil {
		return nil, 0, err
	}
	b := logsegment.GetRootAsRecordBatch(f.data[4:], 0)
	batch := &RecordBatch{
		input:      r.input,
		byteOffset: byteOffset,
		byteLength: int64(len(f.data)),
		header:     header,
		batch:      *b,
		frame:      f,
	}
	return batch, int64(len(f.data)) + int64(b.CompressedSize()), nil
}

func (r *LogReader) readFrameAt(byteOffset int64) (*buffer, error) {
	f := frameBufferPool.get(int(r.bufferSize))

	n, err := r.input.ReadAt(f.data, byteOffset)
	if n < 4 {
		if err == io.EOF {
			if n == 0 {
				return nil, err
			}
			err = io.ErrUnexpectedEOF
		}
		frameBufferPool.put(f)
		return nil, fmt.Errorf("reading log segment frame at offset %d: %w", byteOffset, err)
	}

	frameSize := binary.LittleEndian.Uint32(f.data[:4])
	if frameSize > maxFrameSize {
		frameBufferPool.put(f)
		return nil, fmt.Errorf("log segment frame at offset %d is too large (%d>%d)", byteOffset, frameSize, maxFrameSize)
	}

	byteLength := int(4 + frameSize)
	if n >= byteLength {
		f.data = f.data[:byteLength]
		return f, nil
	}

	if cap(f.data) >= byteLength {
		f.data = f.data[:byteLength]
	} else {
		defer frameBufferPool.put(f)
		newFrame := newBuffer(byteLength)
		copy(newFrame.data, f.data)
		f = newFrame
	}

	if _, err := r.input.ReadAt(f.data[n:], byteOffset+int64(n)); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		frameBufferPool.put(f)
		return nil, fmt.Errorf("reading %dB log segment frame at offset %d: %w", byteLength, byteOffset, err)
	}
	return f, nil
}

type buffer struct{ data []byte }

type bufferPool struct{ pool sync.Pool }

func (p *bufferPool) get(size int) *buffer {
	b, _ := p.pool.Get().(*buffer)
	if b != nil {
		if size <= cap(b.data) {
			b.data = b.data[:size]
			return b
		}
		p.put(b)
		b = nil
	}
	return newBuffer(size)
}

func (p *bufferPool) put(b *buffer) {
	if b != nil {
		p.pool.Put(b)
	}
}

var (
	frameBufferPool        bufferPool
	compressedBufferPool   bufferPool
	uncompressedBufferPool bufferPool
)

func newBuffer(size int) *buffer {
	return &buffer{data: make([]byte, size, align(size))}
}

func releaseBuffer(buf **buffer, pool *bufferPool) {
	if b := *buf; b != nil {
		*buf = nil
		pool.put(b)
	}
}

func align(size int) int {
	return ((size + (defaultBufferSize - 1)) / defaultBufferSize) * defaultBufferSize
}

type objectPool[T any] struct {
	pool sync.Pool
}

func (p *objectPool[T]) get(newObject func() T) T {
	v, ok := p.pool.Get().(T)
	if ok {
		return v
	}
	return newObject()
}

func (p *objectPool[T]) put(obj T) {
	p.pool.Put(obj)
}

var (
	zstdEncoderPool objectPool[*zstd.Encoder]
	zstdDecoderPool objectPool[*zstd.Decoder]
)

func compress(dst, src []byte, compression Compression) []byte {
	switch compression {
	case Snappy:
		return snappy.Encode(dst, src)
	case Zstd:
		enc := zstdEncoderPool.get(func() *zstd.Encoder {
			e, _ := zstd.NewWriter(nil,
				zstd.WithEncoderCRC(false),
				zstd.WithEncoderConcurrency(1),
				zstd.WithEncoderLevel(zstd.SpeedFastest),
			)
			return e
		})
		defer zstdEncoderPool.put(enc)
		return enc.EncodeAll(src, dst[:0])
	default:
		return append(dst[:0], src...)
	}
}

func decompress(dst, src []byte, compression Compression) ([]byte, error) {
	switch compression {
	case Snappy:
		return snappy.Decode(dst, src)
	case Zstd:
		dec := zstdDecoderPool.get(func() *zstd.Decoder {
			d, _ := zstd.NewReader(nil,
				zstd.IgnoreChecksum(true),
				zstd.WithDecoderConcurrency(1),
			)
			return d
		})
		defer zstdDecoderPool.put(dec)
		return dec.DecodeAll(src, dst[:0])
	default:
		return dst, fmt.Errorf("unknown compression format: %d", compression)
	}
}

// LogWriter supports writing log segments to an io.Writer.
//
// The type has two main methods, WriteLogHeader and WriteRecordBatch.
// The former must be called first and only once to write the log header and
// initialize the state of the writer, zero or more record batches may then
// be written to the log after that.
type LogWriter struct {
	output       io.Writer
	builder      *flatbuffers.Builder
	uncompressed []byte
	compressed   []byte
	// The compression format declared in the log header first written to the
	// log segment.
	compression Compression
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
	memory  []flatbuffers.UOffsetT
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
		output:       output,
		builder:      flatbuffers.NewBuilder(bufferSize),
		uncompressed: make([]byte, 0, bufferSize),
	}
}

// Reset resets the state of the log writer to produce to output to the given
// io.Writer.
//
// WriteLogHeader should be called again after resetting the writer.
func (w *LogWriter) Reset(output io.Writer) {
	w.output = output
	w.builder.Reset()
	w.uncompressed = w.uncompressed[:0]
	w.compressed = w.compressed[:0]
	w.compression = Uncompressed
	w.startTime = time.Time{}
	w.nextOffset = 0
	w.stickyErr = nil
	w.memory = w.memory[:0]
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

	functions := w.prependObjectVector(len(functionOffsets),
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
	logsegment.LogHeaderAddCompression(w.builder, header.Compression)
	logsegment.FinishSizePrefixedLogHeaderBuffer(w.builder, logsegment.LogHeaderEnd(w.builder))

	if _, err := w.output.Write(w.builder.FinishedBytes()); err != nil {
		w.stickyErr = err
		return err
	}
	w.startTime = header.Process.StartTime
	w.compression = header.Compression
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
	w.uncompressed = w.uncompressed[:0]
	w.compressed = w.compressed[:0]
	w.records = w.records[:0]

	for _, record := range batch {
		w.memory = w.memory[:0]
		for _, m := range record.MemoryAccess {
			memoryBytesOffset := w.builder.CreateByteVector(m.Memory)
			logsegment.MemoryAccessStart(w.builder)
			logsegment.MemoryAccessAddMemory(w.builder, memoryBytesOffset)
			logsegment.MemoryAccessAddOffset(w.builder, m.Offset)
			w.memory = append(w.memory, logsegment.MemoryAccessEnd(w.builder))
		}
		memory := w.prependOffsetVector(w.memory)

		params := w.prependUint64Vector(record.Params)
		results := w.prependUint64Vector(record.Results)
		timestamp := int64(record.Timestamp.Sub(w.startTime))
		function := uint32(record.Function)

		logsegment.RecordStart(w.builder)
		logsegment.RecordAddTimestamp(w.builder, timestamp)
		logsegment.RecordAddFunction(w.builder, function)
		logsegment.RecordAddParams(w.builder, params)
		logsegment.RecordAddResults(w.builder, results)
		logsegment.RecordAddMemoryAccess(w.builder, memory)

		w.records = append(w.records, logsegment.RecordEnd(w.builder))
	}

	firstOffset := w.nextOffset
	w.nextOffset += int64(len(batch))

	records := w.prependOffsetVector(w.records)
	memory := w.uncompressed
	if w.compression != Uncompressed {
		w.compressed = compress(w.compressed[:cap(w.compressed)], memory, w.compression)
		memory = w.compressed
	}

	logsegment.RecordBatchStart(w.builder)
	logsegment.RecordBatchAddFirstOffset(w.builder, firstOffset)
	logsegment.RecordBatchAddCompressedSize(w.builder, uint32(len(memory)))
	logsegment.RecordBatchAddUncompressedSize(w.builder, uint32(len(w.uncompressed)))
	logsegment.RecordBatchAddChecksum(w.builder, 0)
	logsegment.RecordBatchAddRecords(w.builder, records)
	w.builder.FinishSizePrefixed(logsegment.RecordBatchEnd(w.builder))

	if _, err := w.output.Write(w.builder.FinishedBytes()); err != nil {
		w.stickyErr = err
		return firstOffset, err
	}
	if _, err := w.output.Write(memory); err != nil {
		w.stickyErr = err
		return firstOffset, err
	}
	return firstOffset, nil
}

func (w *LogWriter) prependHash(hash Hash) flatbuffers.UOffsetT {
	return hash.prepend(w.builder)
}

func (w *LogWriter) prependStringVector(values []string) flatbuffers.UOffsetT {
	return w.prependObjectVector(len(values), func(i int) flatbuffers.UOffsetT {
		return w.builder.CreateString(values[i])
	})
}

func (w *LogWriter) prependUint64Vector(values []uint64) flatbuffers.UOffsetT {
	return prependUint64Vector(w.builder, values)
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
	w.builder.StartVector(flatbuffers.SizeUOffsetT, len(offsets), flatbuffers.SizeUOffsetT)
	for i := len(offsets) - 1; i >= 0; i-- {
		w.builder.PrependUOffsetT(offsets[i])
	}
	return w.builder.EndVector(len(offsets))
}

// BufferedLogWriter wraps a LogWriter to help with write batching.
//
// A single WriteRecord method is provided for writing records. When the number
// of buffered records reaches the configured batch size, the batch is passed
// to the LogWriter's WriteRecordBatch method.
type BufferedLogWriter struct {
	*LogWriter

	batchSize int
	batch     []Record

	memoryAccesses []MemoryAccess
	buffer         []byte
}

// NewBufferedLogWriter creates a BufferedLogWriter.
func NewBufferedLogWriter(w *LogWriter, batchSize int) *BufferedLogWriter {
	return &BufferedLogWriter{
		LogWriter: w,
		batchSize: batchSize,
		batch:     make([]Record, 0, batchSize),
		buffer:    make([]byte, 0, 4096),
	}
}

// WriteRecord buffers a Record and then writes it once the internal
// record batch is full.
//
// The writer will make a copy of the memory accesses.
func (w *BufferedLogWriter) WriteRecord(record Record) error {
	for i := range record.MemoryAccess {
		m := &record.MemoryAccess[i]
		w.buffer = append(w.buffer, m.Memory...)
		w.memoryAccesses = append(w.memoryAccesses, MemoryAccess{
			Offset: m.Offset,
			Memory: w.buffer[len(w.buffer)-len(m.Memory):],
		})
	}
	record.MemoryAccess = w.memoryAccesses[len(w.memoryAccesses)-len(record.MemoryAccess):]

	w.batch = append(w.batch, record)
	if len(w.batch) < w.batchSize {
		return nil
	}
	return w.Flush()
}

// Flush flushes any pending records.
func (w *BufferedLogWriter) Flush() error {
	if len(w.batch) == 0 {
		return nil
	}
	if _, err := w.WriteRecordBatch(w.batch); err != nil {
		return err
	}
	w.batch = w.batch[:0]
	w.buffer = w.buffer[:0]
	w.memoryAccesses = w.memoryAccesses[:0]
	return nil
}

// LogRecordIterator is a helper for iterating records in a log.
type LogRecordIterator struct {
	reader       *LogReader
	header       *LogHeader
	batch        *RecordBatch
	record       RecordReader
	batchIndex   int
	readerOffset int64
	err          error
}

// NewLogRecordIterator creates a log record iterator.
func NewLogRecordIterator(r *LogReader) *LogRecordIterator {
	return &LogRecordIterator{reader: r}
}

// Next is true if there is another Record available.
func (i *LogRecordIterator) Next() bool {
	if i.header == nil {
		i.header, i.readerOffset, i.err = i.reader.ReadLogHeader()
		if i.err != nil {
			return false
		}
	}
	if i.batch == nil || i.batchIndex == i.batch.NumRecords() {
		if i.batch != nil {
			i.batch.Close()
		}
		i.batchIndex = 0
		var batchLength int64
		i.batch, batchLength, i.err = i.reader.ReadRecordBatch(i.header, i.readerOffset)
		if i.err != nil {
			return false
		}
		i.readerOffset += batchLength
		if i.batch.NumRecords() == 0 {
			return false
		}
	}
	i.record = i.batch.RecordReaderAt(i.batchIndex)
	i.batchIndex++
	return true
}

// Error returns any errors during reads or the preparation of records.
func (i *LogRecordIterator) Error() error {
	return i.err
}

// Record returns the next record as a RecordReader.
//
// The return value is only valid when Next returns true.
func (i *LogRecordIterator) Record() RecordReader {
	return i.record
}

func (i *LogRecordIterator) Close() error {
	if i.batch != nil {
		i.batch.Close()
		i.batch = nil
	}
	return i.err
}
