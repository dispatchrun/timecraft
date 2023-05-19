package timemachine

import (
	"io"

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
	output  io.Writer
	builder *flatbuffers.Builder
	// When writing to the underlying io.Writer causes an error, we stop
	// accepting writes and assume the log is corrupted.
	stickyErr error

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
	}
}

// Reset resets the state of the log writer to produce to output to the given
// io.Writer.
//
// WriteLogHeader should be called again after resetting the writer.
func (w *LogWriter) Reset(output io.Writer) {
	w.output = output
	w.builder.Reset()
	w.stickyErr = nil
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
	return nil
}

// WriteRecordBatch writes a record batch to the log. The method returns
// a non-nil error if the write failed.
//
// If the error occurred while writing to the underlying io.Writer, the writer
// is broken and will always error on future calls to WriteRecordBatch until
// the program calls Reset.
func (w *LogWriter) WriteRecordBatch(batch RecordBatchBuilder) error {
	if w.stickyErr != nil {
		return w.stickyErr
	}
	_, err := batch.Write(w.output)
	return err
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

// LogRecordWriter wraps a LogWriter to help with write batching.
//
// A WriteRecord method is added that buffers records in a batch up to a
// configurable size before flushing the batch to the log.
type LogRecordWriter struct {
	*LogWriter

	batchSize   int
	compression Compression
	firstOffset int64
	batch       RecordBatchBuilder
	count       int
}

// NewLogRecordWriter creates a LogRecordWriter.
func NewLogRecordWriter(w *LogWriter, batchSize int, compression Compression) *LogRecordWriter {
	bw := &LogRecordWriter{
		LogWriter:   w,
		compression: compression,
		batchSize:   batchSize,
	}
	bw.batch.Reset(compression, 0)
	return bw
}

// WriteRecord buffers a Record in a batch and then flushes the batch once
// it reaches the configured maximum size.
func (w *LogRecordWriter) WriteRecord(record RecordBuilder) error {
	w.batch.AddRecord(record)
	w.count++
	if w.count >= w.batchSize {
		return w.Flush()
	}
	return nil
}

// Flush flushes the pending batch.
func (w *LogRecordWriter) Flush() error {
	if w.count == 0 {
		return nil
	}
	if err := w.WriteRecordBatch(w.batch); err != nil {
		return err
	}
	w.firstOffset += int64(w.count)
	w.count = 0
	w.batch.Reset(w.compression, w.firstOffset)
	return nil
}
