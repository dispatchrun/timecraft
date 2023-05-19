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
	Module      string
	Name        string
	ParamCount  int
	ResultCount int
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
	frame   *buffer
	records *buffer

	// Loading of the records buffer is synchronized on this once value so it may
	// happen when records are read concurrently.
	//
	// If an error occurs while reading, it is captured in `err` and all reads of
	// the records will observe the error.
	once sync.Once
	err  error

	// When using the batch as a record iterator, this holds the current offset
	// into the records.
	offset uint32
	record RecordReader
}

// Close must be called by the application when the batch isn't needed anymore
// to allow the resources held internally to be reused by the application.
func (b *RecordBatch) Close() error {
	b.once.Do(func() {})
	recordsBufferPool := &compressedBufferPool
	if b.header.Compression == Uncompressed {
		recordsBufferPool = &uncompressedBufferPool
	}
	releaseBuffer(&b.records, recordsBufferPool)
	releaseBuffer(&b.frame, &frameBufferPool)
	b.batch = logsegment.RecordBatch{}
	return nil
}

// Compression returns the compression algorithm used to encode the record
// batch data section.
func (b *RecordBatch) Compression() Compression {
	return b.header.Compression
}

// FirstOffset returns the logical offset of the first record in the batch.
func (b *RecordBatch) FirstOffset() int64 {
	return b.batch.FirstOffset()
}

// FirstTimestamp returns the time at which the first record was produced.
func (b *RecordBatch) FirstTimestamp() time.Time {
	return b.header.Process.StartTime.Add(time.Duration(b.batch.FirstTimestamp()))
}

// CompressedSize returns the size of the record batch data section in the log
// segment.
func (b *RecordBatch) CompressedSize() int64 {
	return int64(b.batch.CompressedSize())
}

// UncompressedSize returns the size of the record batch data section after
// being uncompressed.
func (b *RecordBatch) UncompressedSize() int64 {
	return int64(b.batch.UncompressedSize())
}

// NumRecords returns the number of records in the batch.
func (b *RecordBatch) NumRecords() int {
	return int(b.batch.NumRecords())
}

// Records is a helper function which reads all records of a batch in memory.
//
// The method is useful in contexts with relaxed performance constraints, as the
// returned values are heap allocated and hold copies of the underlying memory
// buffers.
func (b *RecordBatch) Records() ([]Record, error) {
	records := make([]Record, 0, b.NumRecords())
	b.Rewind()
	for b.Next() {
		rr, err := b.Record()
		if err != nil {
			return nil, err
		}
		records = append(records, Record{
			Timestamp:    rr.Timestamp(),
			Function:     rr.FunctionId(),
			Params:       rr.Params(),
			Results:      rr.Results(),
			MemoryAccess: rr.MemoryAccess(),
		})
	}
	return records, nil
}

// Next is true if the batch contains another record.
func (b *RecordBatch) Next() bool {
	records, err := b.loadRecords()
	if err != nil {
		return true // the error is returned on the Record() call
	}
	if b.offset >= uint32(len(records)) {
		return false
	}
	offset := b.offset
	if offset+4 < offset || offset+4 > uint32(len(records)) {
		b.err = fmt.Errorf("cannot read record at offset %d as records buffer is length %d", offset, len(records))
		return true
	}
	size := binary.LittleEndian.Uint32(records[offset:])
	if offset+size < offset || offset+size > uint32(len(records)) {
		b.err = fmt.Errorf("cannot read record at [%d:%d+%d] as records buffer is length %d", offset, offset, size, len(records))
		return true
	}
	record := logsegment.GetRootAsRecord(records[offset+4:], 0)
	functionCall := logsegment.GetRootAsFunctionCall(record.FunctionCallBytes(), 0)
	b.record = RecordReader{
		batch:        b,
		record:       *record,
		functionCall: *functionCall,
	}
	b.offset += size + 4
	return true
}

// Rewind rewinds the batch in order to read records again.
func (b *RecordBatch) Rewind() {
	b.offset = 0
}

// Record returns the next record in the batch.
func (b *RecordBatch) Record() (RecordReader, error) {
	return b.record, b.err
}

func (b *RecordBatch) loadRecords() ([]byte, error) {
	b.once.Do(func() {
		b.records, b.err = b.readRecords()
	})
	if b.err != nil {
		return nil, b.err
	}
	return b.records.data, nil
}

func (b *RecordBatch) readRecords() (*buffer, error) {
	compression := b.header.Compression
	compressedSize := int(b.CompressedSize())
	uncompressedSize := int(b.UncompressedSize())

	var recordsBufferPool *bufferPool
	var recordsBufferSize int
	if compression == Uncompressed {
		recordsBufferPool = &uncompressedBufferPool
		recordsBufferSize = uncompressedSize
	} else {
		recordsBufferPool = &compressedBufferPool
		recordsBufferSize = compressedSize
	}

	recordsBuffer := recordsBufferPool.get(recordsBufferSize)

	_, err := b.input.ReadAt(recordsBuffer.data, b.byteOffset+b.byteLength)
	if err != nil {
		recordsBufferPool.put(recordsBuffer)
		return nil, err
	}

	if c := checksum(recordsBuffer.data); c != b.batch.Checksum() {
		return nil, fmt.Errorf("bad record data: expect checksum %#x, got %#x", b.batch.Checksum(), c)
	}

	if compression == Uncompressed {
		return recordsBuffer, nil
	}
	defer recordsBufferPool.put(recordsBuffer)

	uncompressedMemoryBuffer := uncompressedBufferPool.get(uncompressedSize)
	src := recordsBuffer.data
	dst := uncompressedMemoryBuffer.data
	dst, err = decompress(dst, src, compression)
	uncompressedMemoryBuffer.data = dst
	return uncompressedMemoryBuffer, err
}

// RecordReader values are returned by calling RecordReaderAt on a RecordBatch,
// which lets the program read a single record from a batch.
type RecordReader struct {
	batch        *RecordBatch
	record       logsegment.Record
	functionCall logsegment.FunctionCall
}

// Timestamp returns the time at which the record was produced.
func (r *RecordReader) Timestamp() time.Time {
	return r.batch.header.Process.StartTime.Add(time.Duration(r.record.Timestamp()))
}

// FunctionId returns the ID of the function that produced the record.
func (r *RecordReader) FunctionId() int {
	return int(r.record.FunctionId())
}

// LookupFunction returns a Function object representing the function that
// produced the record.
func (r *RecordReader) LookupFunction() (Function, bool) {
	if i := r.FunctionId(); i >= 0 && i < len(r.batch.header.Runtime.Functions) {
		return r.batch.header.Runtime.Functions[i], true
	}
	return Function{}, false
}

// NumParams returns the number of parameters that were passed to the function.
func (r *RecordReader) NumParams() int {
	fn, ok := r.LookupFunction()
	if !ok {
		panic("LookupFunction")
	}
	return fn.ParamCount
}

// ParamAt returns the param at the specified index.
func (r *RecordReader) ParamAt(i int) uint64 {
	return r.functionCall.Stack(i)
}

// Params returns the function parameters as a newly allocated slice.
func (r *RecordReader) Params() []uint64 {
	numParams := r.NumParams()
	if numParams == 0 {
		return nil
	}
	params := make([]uint64, numParams)
	for i := range params {
		params[i] = r.ParamAt(i)
	}
	return params
}

// NumResults returns the number of results that were returned by the function.
func (r *RecordReader) NumResults() int {
	return r.functionCall.StackLength() - r.NumParams()
}

// ReadResults reads the function results into the slice passed as argument.
func (r *RecordReader) ReadResults(results []uint64) {
}

// ResultAt returns the param at the specified index.
func (r *RecordReader) ResultAt(i int) uint64 {
	paramCount := r.NumParams()
	return r.functionCall.Stack(paramCount + i)
}

// Results returns the function results as a newly allocated slice.
func (r *RecordReader) Results() []uint64 {
	numResults := r.NumResults()
	if numResults == 0 {
		return nil
	}
	results := make([]uint64, numResults)
	for i := range results {
		results[i] = r.ResultAt(i)
	}
	return results
}

// NumMemoryAccess returns the number of memory access recorded in r.
func (r *RecordReader) NumMemoryAccess() int {
	return int(r.functionCall.MemoryAccessLength())
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
	r.functionCall.MemoryAccess(&m, i)
	b := r.functionCall.MemoryBytes()
	offset, length := m.IndexOffset(), m.Length()
	return MemoryAccess{
		Memory: b[offset : offset+length : offset+length],
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
			Module:      string(f.Module()),
			Name:        string(f.Name()),
			ParamCount:  int(f.ParamCount()),
			ResultCount: int(f.ResultCount()),
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
	output              io.Writer
	builder             *flatbuffers.Builder
	functionCallBuilder *flatbuffers.Builder
	uncompressed        []byte
	compressed          []byte
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
	memory  []byte
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
		output:              output,
		builder:             flatbuffers.NewBuilder(bufferSize),
		functionCallBuilder: flatbuffers.NewBuilder(bufferSize),
		uncompressed:        make([]byte, 0, bufferSize),
	}
}

// Reset resets the state of the log writer to produce to output to the given
// io.Writer.
//
// WriteLogHeader should be called again after resetting the writer.
func (w *LogWriter) Reset(output io.Writer) {
	w.output = output
	w.builder.Reset()
	w.functionCallBuilder.Reset()
	w.uncompressed = w.uncompressed[:0]
	w.compressed = w.compressed[:0]
	w.compression = Uncompressed
	w.startTime = time.Time{}
	w.nextOffset = 0
	w.stickyErr = nil
	w.memory = w.memory[:0]
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
		module, name            flatbuffers.UOffsetT
		paramCount, resultCount uint32
	}

	functionOffsets := make([]function, len(header.Runtime.Functions))
	for i, fn := range header.Runtime.Functions {
		functionOffsets[i] = function{
			module:      w.builder.CreateSharedString(fn.Module),
			name:        w.builder.CreateString(fn.Name),
			paramCount:  uint32(fn.ParamCount),
			resultCount: uint32(fn.ResultCount),
		}
	}

	functions := w.prependObjectVector(len(functionOffsets),
		func(i int) flatbuffers.UOffsetT {
			logsegment.FunctionStart(w.builder)
			logsegment.FunctionAddModule(w.builder, functionOffsets[i].module)
			logsegment.FunctionAddName(w.builder, functionOffsets[i].name)
			logsegment.FunctionAddParamCount(w.builder, functionOffsets[i].paramCount)
			logsegment.FunctionAddResultCount(w.builder, functionOffsets[i].resultCount)
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
	w.uncompressed = w.uncompressed[:0]
	w.compressed = w.compressed[:0]

	for _, record := range batch {
		w.functionCallBuilder.Reset()
		w.memory = w.memory[:0]
		logsegment.FunctionCallStartMemoryAccessVector(w.functionCallBuilder, len(record.MemoryAccess))
		w.functionCallBuilder.Prep(12*len(record.MemoryAccess), 0)
		for i := len(record.MemoryAccess) - 1; i >= 0; i-- {
			m := &record.MemoryAccess[i]
			w.functionCallBuilder.PlaceUint32(uint32(len(w.memory))) // offset into the index
			w.functionCallBuilder.PlaceUint32(m.Offset)              // offset into guest memory
			w.functionCallBuilder.PlaceUint32(uint32(len(m.Memory))) // length of captured memory
			w.memory = append(w.memory, m.Memory...)
		}
		memoryAccess := w.functionCallBuilder.EndVector(len(record.MemoryAccess))
		memory := w.functionCallBuilder.CreateByteVector(w.memory)

		w.functionCallBuilder.StartVector(flatbuffers.SizeUint64, len(record.Params)+len(record.Results), flatbuffers.SizeUint64)
		for i := len(record.Results) - 1; i >= 0; i-- {
			w.functionCallBuilder.PrependUint64(record.Results[i])
		}
		for i := len(record.Params) - 1; i >= 0; i-- {
			w.functionCallBuilder.PrependUint64(record.Params[i])
		}
		stack := w.functionCallBuilder.EndVector(len(record.Params) + len(record.Results))

		timestamp := int64(record.Timestamp.Sub(w.startTime))
		function := uint32(record.Function)

		logsegment.FunctionCallStart(w.functionCallBuilder)
		logsegment.FunctionCallAddStack(w.functionCallBuilder, stack)
		logsegment.FunctionCallAddMemoryAccess(w.functionCallBuilder, memoryAccess)
		logsegment.FunctionCallAddMemory(w.functionCallBuilder, memory)
		logsegment.FinishFunctionCallBuffer(w.functionCallBuilder, logsegment.FunctionCallEnd(w.functionCallBuilder))

		w.builder.Reset()
		functionCall := w.builder.CreateByteVector(w.functionCallBuilder.FinishedBytes())

		logsegment.RecordStart(w.builder)
		logsegment.RecordAddTimestamp(w.builder, timestamp)
		logsegment.RecordAddFunctionId(w.builder, function)
		logsegment.RecordAddFunctionCall(w.builder, functionCall)
		logsegment.FinishSizePrefixedRecordBuffer(w.builder, logsegment.RecordEnd(w.builder))

		w.uncompressed = append(w.uncompressed, w.builder.FinishedBytes()...)
	}

	firstOffset := w.nextOffset
	w.nextOffset += int64(len(batch))

	var firstTimestamp int64
	if len(batch) > 0 {
		firstTimestamp = int64(batch[0].Timestamp.Sub(w.startTime))
	}

	if w.compression != Uncompressed {
		w.compressed = compress(w.compressed[:cap(w.compressed)], w.uncompressed, w.compression)
	}

	w.builder.Reset()

	logsegment.RecordBatchStart(w.builder)
	logsegment.RecordBatchAddFirstOffset(w.builder, firstOffset)
	logsegment.RecordBatchAddFirstTimestamp(w.builder, firstTimestamp)
	logsegment.RecordBatchAddCompressedSize(w.builder, uint32(len(w.compressed)))
	logsegment.RecordBatchAddUncompressedSize(w.builder, uint32(len(w.uncompressed)))
	logsegment.RecordBatchAddChecksum(w.builder, checksum(w.compressed))
	logsegment.RecordBatchAddNumRecords(w.builder, uint32(len(batch)))
	logsegment.FinishSizePrefixedRecordBatchBuffer(w.builder, logsegment.RecordBatchEnd(w.builder))

	if _, err := w.output.Write(w.builder.FinishedBytes()); err != nil {
		w.stickyErr = err
		return firstOffset, err
	}
	if _, err := w.output.Write(w.compressed); err != nil {
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
	for i.batch == nil || !i.batch.Next() {
		if i.batch != nil {
			i.batch.Close()
		}
		var batchLength int64
		i.batch, batchLength, i.err = i.reader.ReadRecordBatch(i.header, i.readerOffset)
		if i.err != nil {
			return false
		}
		i.readerOffset += batchLength
	}
	i.record, i.err = i.batch.Record()
	return i.err == nil
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
