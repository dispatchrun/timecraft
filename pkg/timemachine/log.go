package timemachine

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"time"

	capnp "capnproto.org/go/capnp/v3"
	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/pkg/format"
	"github.com/stealthrocket/timecraft/pkg/format/logsegment"
	"github.com/stealthrocket/timecraft/pkg/format/types"
)

type Hash struct {
	Algorithm, Digest string
}

func makeHash(h *types.Hash) Hash {
	return Hash{
		Algorithm: string(h.Algorithm()),
		Digest:    string(h.Digest()),
	}
}

// type Compression = types.Compression
type Compression = format.Compression

const (
	//Uncompressed Compression = types.CompressionUncompressed
	//Snappy       Compression = types.CompressionSnappy
	//Zstd         Compression = types.CompressionZstd
	Uncompressed Compression = format.Compression_uncompressed
	Snappy       Compression = format.Compression_snappy
	Zstd         Compression = format.Compression_zstd
)

type MemoryAccessType = logsegment.MemoryAccessType

const (
	MemoryRead  MemoryAccessType = logsegment.MemoryAccessTypeMemoryRead
	MemoryWrite MemoryAccessType = logsegment.MemoryAccessTypeMemoryWrite
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

	errMissingRuntime          = errors.New("missing runtime in log header")
	errMissingProcess          = errors.New("missing process in log header")
	errMissingProcessID        = errors.New("missing process id in log header")
	errMissingProcessImage     = errors.New("missing process image in log header")
	errMissingProcessStartTime = errors.New("missing process start time in log header")
)

const (
	defaultBufferSize = 16 * 1024
)

// LogReader instances allow programs to read the content of a record log.
//
// The LogReader type has two main methods, ReadLogHeader and ReadRecordBatch.
// ReadLogHeader should be called first to load the header and initialize the
// reader state. ReadRecrdBatch may be called multiple times until io.EOF is
// returned to scan through the log.
type LogReader struct {
	input       *bufio.Reader
	decoder     *capnp.Decoder
	frame       []byte
	startTime   time.Time
	compression Compression
}

// NewLogReader construct a new log reader consuming input from the given
// io.Reader.
func NewLogReader(input io.Reader) *LogReader {
	return NewLogReaderSize(input, defaultBufferSize)
}

// NewLogReaderSize is like NewLogReader but it allows the program to configure
// the read buffer size.
func NewLogReaderSize(input io.Reader, bufferSize int) *LogReader {
	return &LogReader{
		input:   bufio.NewReaderSize(input, bufferSize),
		decoder: capnp.NewDecoder(input),
	}
}

// Reset clears the reader state and sets it to consume input from the given
// io.Reader.
func (r *LogReader) Reset(input io.Reader) {
	r.input.Reset(input)
	r.decoder = capnp.NewDecoder(input)
	r.startTime = time.Time{}
	r.compression = Uncompressed
}

// ReadLogHeader reads and returns the log header from r.
//
// This method must be called once when the reader is positioned at the
// beginning of the log.
func (r *LogReader) ReadLogHeader() (h *LogHeader, err error) {
	defer func() { err = handle(recover()) }()
	h = new(LogHeader)

	//b := must(r.readFrame())
	//m := must(capnp.Unmarshal(b))
	m := must(r.decoder.Decode())

	header := must(format.ReadRootLogHeader(m))
	runtime := must(header.Runtime())

	h.Runtime.Runtime = must(runtime.Runtime())
	h.Runtime.Version = must(runtime.Version())

	functions := must(runtime.Functions())
	h.Runtime.Functions = make([]Function, functions.Len())

	for i := range h.Runtime.Functions {
		f := functions.At(i)
		h.Runtime.Functions[i] = Function{
			Module: must(f.Module()),
			Name:   must(f.Name()),
		}
	}

	process := must(header.Process())
	h.Process.ID = readHash(must(process.Id()))
	h.Process.Image = readHash(must(process.Image()))
	h.Process.StartTime = time.Unix(0, process.UnixStartTime())
	h.Process.Args = readStringArray(must(process.Arguments()))
	h.Process.Environ = readStringArray(must(process.Environment()))

	if process.HasParentProcessId() {
		h.Process.ParentProcessID = readHash(must(process.ParentProcessId()))
		h.Process.ParentForkOffset = process.ParentForkOffset()
	}

	h.Segment = header.SegmentNumber()
	h.Compression = header.Compression()
	return h, nil

	// var h LogHeader
	// var header = logsegment.GetRootAsLogHeader(b, 0)
	// var runtime logsegment.Runtime
	// if header.Runtime(&runtime) == nil {
	// 	return nil, errMissingRuntime
	// }
	// h.Runtime.Runtime = string(runtime.Runtime())
	// h.Runtime.Version = string(runtime.Version())
	// h.Runtime.Functions = make([]Function, runtime.FunctionsLength())

	// for i := range h.Runtime.Functions {
	// 	f := logsegment.Function{}
	// 	if !runtime.Functions(&f, i) {
	// 		return nil, fmt.Errorf("missing runtime function in log header: expected %d but could not load function at index %d", len(h.Runtime.Functions), i)
	// 	}
	// 	h.Runtime.Functions[i] = Function{
	// 		Module: string(f.Module()),
	// 		Name:   string(f.Name()),
	// 	}
	// }

	// var hash types.Hash
	// var process logsegment.Process
	// if header.Process(&process) == nil {
	// 	return nil, errMissingProcess
	// }
	// if process.Id(&hash) == nil {
	// 	return nil, errMissingProcessID
	// }
	// h.Process.ID = makeHash(&hash)
	// if process.Image(&hash) == nil {
	// 	return nil, errMissingProcessImage
	// }
	// h.Process.Image = makeHash(&hash)
	// if unixStartTime := process.UnixStartTime(); unixStartTime == 0 {
	// 	return nil, errMissingProcessStartTime
	// } else {
	// 	h.Process.StartTime = time.Unix(0, unixStartTime)
	// }
	// h.Process.Args = make([]string, process.ArgumentsLength())
	// for i := range h.Process.Args {
	// 	h.Process.Args[i] = string(process.Arguments(i))
	// }
	// h.Process.Environ = make([]string, process.EnvironmentLength())
	// for i := range h.Process.Environ {
	// 	h.Process.Environ[i] = string(process.Environment(i))
	// }
	// if process.ParentProcessId(&hash) != nil {
	// 	h.Process.ParentProcessID = makeHash(&hash)
	// 	h.Process.ParentForkOffset = process.ParentForkOffset()
	// }
	// h.Segment = header.Segment()
	// h.Compression = header.Compression()

	// r.startTime = h.Process.StartTime
	// r.compression = h.Compression
	// return &h, nil
}

type RecordBatch struct {
	logsegment.RecordBatch
}

func (b *RecordBatch) FirstOffset() int64 {
	return 0
}

func (r *LogReader) ReadRecordBatch() (*RecordBatch, error) {
	b, err := r.readFrame()
	if err != nil {
		return nil, err
	}
	_ = b
	return nil, io.EOF // TODO
}

func (r *LogReader) readFrame() ([]byte, error) {
	b, err := r.input.Peek(4)
	if err != nil {
		return nil, err
	}
	n := 4 + int(binary.LittleEndian.Uint32(b))
	if n <= cap(r.frame) {
		r.frame = r.frame[:n]
	} else {
		r.frame = make([]byte, n)
	}
	_, err = io.ReadFull(r.input, r.frame)
	return r.frame[4:], err
}

// LogWriter supports writing log segments to an io.Writer.
//
// The type has two main methods, WriteLogHeader and WriteRecordBatch.
// The former must be called first and only once to write the log header and
// initialize the state of the writer, zero or more record batches may then
// be written to the log after that.
type LogWriter struct {
	output  io.Writer
	encoder *capnp.Encoder
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
		encoder: capnp.NewEncoder(output),
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
	w.encoder = capnp.NewEncoder(output)
	w.builder.Reset()
	w.buffer.Reset()
	w.startTime = time.Time{}
	w.nextOffset = 0
	w.stickyErr = nil
	w.records = w.records[:0]
	w.offsets = w.offsets[:0]
}

func (w *LogWriter) writeFrame(b []byte) error {
	n := make([]byte, 4)
	binary.LittleEndian.PutUint32(n, uint32(len(b)))
	if _, err := w.output.Write(n); err != nil {
		w.stickyErr = err
		return err
	}
	if _, err := w.output.Write(b); err != nil {
		w.stickyErr = err
		return err
	}
	return nil
}

// WriteLogHeader writes the log header. The method must be called before any
// records are written to the log via calls to WriteRecordBatch.
func (w *LogWriter) WriteLogHeader(h *LogHeader) (err error) {
	defer func() { err = handle(recover()) }()

	arena := capnp.SingleSegment(nil)
	defer arena.Release()

	msg, seg, err := capnp.NewMessage(arena)
	check(err)

	header, err := format.NewRootLogHeader(seg)
	check(err)
	header.SetSegmentNumber(uint32(h.Segment))
	header.SetCompression(h.Compression)

	runtime, err := header.NewRuntime()
	check(err)
	check(runtime.SetRuntime(h.Runtime.Runtime))
	check(runtime.SetVersion(h.Runtime.Version))

	functions, err := runtime.NewFunctions(int32(len(h.Runtime.Functions)))
	check(err)
	for i, fn := range h.Runtime.Functions {
		f := functions.At(i)
		check(f.SetModule(fn.Module))
		check(f.SetName(fn.Name))
	}

	process, err := header.NewProcess()
	check(err)
	process.SetUnixStartTime(h.Process.StartTime.UnixNano())

	processID, err := process.NewId()
	check(err)
	check(setHash(processID, h.Process.ID))

	processImage, err := process.NewImage()
	check(err)
	check(setHash(processImage, h.Process.Image))

	arguments, err := process.NewArguments(int32(len(h.Process.Args)))
	check(err)
	check(setTextList(arguments, h.Process.Args))

	environment, err := process.NewEnvironment(int32(len(h.Process.Environ)))
	check(err)
	check(setTextList(environment, h.Process.Environ))

	if h.Process.ParentProcessID.Digest != "" {
		parentProcessID, err := process.NewParentProcessId()
		check(err)
		check(setHash(parentProcessID, h.Process.ParentProcessID))
		process.SetParentForkOffset(h.Process.ParentForkOffset)
	}

	check(w.encoder.Encode(msg))
	w.startTime = h.Process.StartTime
	return nil

	// if w.stickyErr != nil {
	// 	return w.stickyErr
	// }
	// w.builder.Reset()

	// processID := w.prependHash(header.Process.ID)
	// processImage := w.prependHash(header.Process.Image)
	// processArguments := w.prependStringVector(header.Process.Args)
	// processEnvironment := w.prependStringVector(header.Process.Environ)

	// var parentProcessID flatbuffers.UOffsetT
	// if header.Process.ParentProcessID.Digest != "" {
	// 	parentProcessID = w.prependHash(header.Process.ParentProcessID)
	// }

	// logsegment.ProcessStart(w.builder)
	// logsegment.ProcessAddId(w.builder, processID)
	// logsegment.ProcessAddImage(w.builder, processImage)
	// logsegment.ProcessAddUnixStartTime(w.builder, header.Process.StartTime.UnixNano())
	// logsegment.ProcessAddArguments(w.builder, processArguments)
	// logsegment.ProcessAddEnvironment(w.builder, processEnvironment)
	// if parentProcessID != 0 {
	// 	logsegment.ProcessAddParentProcessId(w.builder, parentProcessID)
	// 	logsegment.ProcessAddParentForkOffset(w.builder, header.Process.ParentForkOffset)
	// }
	// processOffset := logsegment.ProcessEnd(w.builder)

	// type function struct {
	// 	module, name flatbuffers.UOffsetT
	// }

	// functionOffsets := make([]function, 0, 64)
	// if len(header.Runtime.Functions) <= cap(functionOffsets) {
	// 	functionOffsets = functionOffsets[:len(header.Runtime.Functions)]
	// } else {
	// 	functionOffsets = make([]function, len(header.Runtime.Functions))
	// }

	// for i, fn := range header.Runtime.Functions {
	// 	functionOffsets[i] = function{
	// 		module: w.builder.CreateSharedString(fn.Module),
	// 		name:   w.builder.CreateString(fn.Name),
	// 	}
	// }

	// functions := w.prependObjectVector(len(functionOffsets),
	// 	func(i int) flatbuffers.UOffsetT {
	// 		logsegment.FunctionStart(w.builder)
	// 		logsegment.FunctionAddModule(w.builder, functionOffsets[i].module)
	// 		logsegment.FunctionAddName(w.builder, functionOffsets[i].name)
	// 		return logsegment.FunctionEnd(w.builder)
	// 	},
	// )

	// runtime := w.builder.CreateString(header.Runtime.Runtime)
	// version := w.builder.CreateString(header.Runtime.Version)
	// logsegment.RuntimeStart(w.builder)
	// logsegment.RuntimeAddRuntime(w.builder, runtime)
	// logsegment.RuntimeAddVersion(w.builder, version)
	// logsegment.RuntimeAddFunctions(w.builder, functions)
	// runtimeOffset := logsegment.RuntimeEnd(w.builder)

	// logsegment.LogHeaderStart(w.builder)
	// logsegment.LogHeaderAddRuntime(w.builder, runtimeOffset)
	// logsegment.LogHeaderAddProcess(w.builder, processOffset)
	// logsegment.LogHeaderAddSegment(w.builder, header.Segment)
	// logsegment.LogHeaderAddCompression(w.builder, header.Compression)
	// logHeader := logsegment.LogHeaderEnd(w.builder)

	// w.builder.FinishSizePrefixedWithFileIdentifier(logHeader, tl0)

	// if _, err := w.output.Write(w.builder.FinishedBytes()); err != nil {
	// 	w.stickyErr = err
	// 	return err
	// }
	// w.startTime = header.Process.StartTime
	// return nil
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
		w.builder.PrependUint64(values[i])
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
		w.builder.PrependUOffsetT(offsets[i])
	}
	return w.builder.EndVector(len(offsets))
}

func (w *LogWriter) prependMemoryAccess(offset uint32, access *MemoryAccess) flatbuffers.UOffsetT {
	// return logsegment.CreateMemoryAccess(
	// 	w.builder,
	// 	access.Offset,
	// 	offset,
	// 	uint32(len(access.Memory)),
	// 	access.Access,
	// )
	w.builder.Prep(4, 16)
	w.builder.PlaceUint32(uint32(access.Access))
	w.builder.PlaceUint32(uint32(len(access.Memory)))
	w.builder.PlaceUint32(offset)
	w.builder.PlaceUint32(access.Offset)
	return w.builder.Offset()
}

func readHash(h format.Hash) Hash {
	return Hash{
		Algorithm: must(h.Algorithm()),
		Digest:    must(h.Digest()),
	}
}

func readStringArray(list capnp.TextList) []string {
	values := make([]string, list.Len())
	for i := range values {
		values[i] = must(list.At(i))
	}
	return values
}

func setHash(dst format.Hash, src Hash) error {
	if err := dst.SetAlgorithm(src.Algorithm); err != nil {
		return err
	}
	if err := dst.SetDigest(src.Digest); err != nil {
		return err
	}
	return nil
}

func setTextList(dst capnp.TextList, src []string) error {
	for i, v := range src {
		if err := dst.Set(i, v); err != nil {
			return err
		}
	}
	return nil
}

// Methods of capnp types produce a lot of errors, which makes the code verbose
// with error checks and negatively impacts readability. Error handling is done
// via local panics in those functions to help reduce the amount of error checks
// that have to be written.
type capnpError struct{ err error }

func handle(x any) error {
	switch v := x.(type) {
	case nil:
		return nil
	case error:
		return v
	default:
		panic(v)
	}
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func must[T any](ret T, err error) T {
	check(err)
	return ret
}
