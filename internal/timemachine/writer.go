package timemachine

import (
	"io"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
)

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
func (w *LogWriter) WriteLogHeader(header *Header) error {
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
func (w *LogWriter) WriteRecordBatch(batch []RecordFIXME) (int64, error) {
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

		logsegment.FunctionCallStart(w.functionCallBuilder)
		logsegment.FunctionCallAddStack(w.functionCallBuilder, stack)
		logsegment.FunctionCallAddMemoryAccess(w.functionCallBuilder, memoryAccess)
		logsegment.FunctionCallAddMemory(w.functionCallBuilder, memory)
		logsegment.FinishFunctionCallBuffer(w.functionCallBuilder, logsegment.FunctionCallEnd(w.functionCallBuilder))

		w.builder.Reset()
		functionCall := w.builder.CreateByteVector(w.functionCallBuilder.FinishedBytes())

		logsegment.RecordStart(w.builder)
		logsegment.RecordAddTimestamp(w.builder, int64(record.Timestamp.Sub(w.startTime)))
		logsegment.RecordAddFunctionId(w.builder, uint32(record.Function))
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
	batch     []RecordFIXME

	memoryAccesses []MemoryAccess
	buffer         []byte
}

// NewBufferedLogWriter creates a BufferedLogWriter.
func NewBufferedLogWriter(w *LogWriter, batchSize int) *BufferedLogWriter {
	return &BufferedLogWriter{
		LogWriter: w,
		batchSize: batchSize,
		batch:     make([]RecordFIXME, 0, batchSize),
		buffer:    make([]byte, 0, 4096),
	}
}

// WriteRecord buffers a Record and then writes it once the internal
// record batch is full.
//
// The writer will make a copy of the memory accesses.
func (w *BufferedLogWriter) WriteRecord(record RecordFIXME) error {
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
