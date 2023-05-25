package timemachine

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"time"

	"github.com/stealthrocket/timecraft/internal/buffer"
	"github.com/stealthrocket/timecraft/internal/stream"
)

const maxFrameSize = (1 * 1024 * 1024) - 4

var frameBufferPool buffer.Pool

// LogReader instances allow programs to read the content of a record log.
type LogReader struct {
	input      *bufio.Reader
	bufferSize int
	startTime  time.Time
	batch      RecordBatch
	batchFrame *buffer.Buffer
}

// NewLogReader construct a new log reader consuming input from the given
// io.Reader.
func NewLogReader(input io.Reader, startTime time.Time) *LogReader {
	return NewLogReaderSize(input, startTime, buffer.DefaultSize)
}

// NewLogReaderSize is like NewLogReader but it allows the program to configure
// the read buffer size.
func NewLogReaderSize(input io.Reader, startTime time.Time, bufferSize int) *LogReader {
	return &LogReader{
		startTime:  startTime,
		input:      bufio.NewReaderSize(input, 64*1024),
		bufferSize: buffer.Align(bufferSize, buffer.DefaultSize),
	}
}

// Close closes the log reader.
func (r *LogReader) Close() error {
	buffer.Release(&r.batchFrame, &frameBufferPool)
	r.batch.Reset(r.startTime, nil, nil)
	return nil
}

// ReadRecordBatch reads the next record batch.
//
// The RecordBatch is only valid until the next call to ReadRecordBatch.
func (r *LogReader) ReadRecordBatch() (*RecordBatch, error) {
	if r.batch.reader.N > 0 {
		if _, err := io.Copy(io.Discard, &r.batch.reader); err != nil {
			return nil, err
		}
	}
	buffer.Release(&r.batchFrame, &frameBufferPool)
	var err error
	r.batchFrame, err = r.readFrame()
	if err != nil {
		return nil, err
	}
	r.batch.Reset(r.startTime, r.batchFrame.Data, r.input)
	return &r.batch, nil
}

func (r *LogReader) readFrame() (*buffer.Buffer, error) {
	f := frameBufferPool.Get(r.bufferSize)

	n, err := io.ReadFull(r.input, f.Data[:4])
	if n < 4 {
		if err == io.EOF {
			if n == 0 {
				return nil, err
			}
			err = io.ErrUnexpectedEOF
		}
		frameBufferPool.Put(f)
		return nil, fmt.Errorf("reading log segment frame: %w", err)
	}

	frameSize := binary.LittleEndian.Uint32(f.Data[:4])
	if frameSize > maxFrameSize {
		frameBufferPool.Put(f)
		return nil, fmt.Errorf("log segment frame is too large (%d>%d)", frameSize, maxFrameSize)
	}

	byteLength := int(4 + frameSize)
	if n >= byteLength {
		f.Data = f.Data[:byteLength]
		return f, nil
	}

	if cap(f.Data) >= byteLength {
		f.Data = f.Data[:byteLength]
	} else {
		defer frameBufferPool.Put(f)
		newFrame := buffer.New(byteLength)
		copy(newFrame.Data, f.Data)
		f = newFrame
	}

	if _, err := io.ReadFull(r.input, f.Data[4:byteLength]); err != nil {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		frameBufferPool.Put(f)
		return nil, fmt.Errorf("reading %dB log segment frame: %w", byteLength, err)
	}
	return f, nil
}

// LogRecordReader wraps a LogReader to help with reading individual records
// in order.
//
// The reader exposes an iterator like interface. Callers should call Next to
// determine whether another record is available. If so, it can be retrieved
// via the Record method.
type LogRecordReader struct {
	reader *LogReader
	batch  *RecordBatch
}

// NewLogRecordReader creates a log record iterator.
func NewLogRecordReader(r *LogReader) *LogRecordReader {
	return &LogRecordReader{reader: r}
}

// Read reads records from r.
//
// The record values share memory buffer with the reader, they remain valid
// until the next call to Read.
func (r *LogRecordReader) Read(records []Record) (int, error) {
	for {
		if r.batch == nil {
			b, err := r.reader.ReadRecordBatch()
			if err != nil {
				return 0, err
			}
			r.batch = b
		}
		n, err := r.batch.Read(records)
		if n > 0 {
			return n, nil
		}
		if err != io.EOF {
			return n, err
		} else {
			r.batch = nil
		}
	}
}

var (
	_ stream.Reader[Record] = (*LogRecordReader)(nil)
)

// LogWriter supports writing log segments to an io.Writer.
type LogWriter struct {
	output io.Writer
	// When writing to the underlying io.Writer causes an error, we stop
	// accepting writes and assume the log is corrupted.
	stickyErr error
}

// NewLogWriter constructs a new log writer which produces output to the given
// io.Writer.
func NewLogWriter(output io.Writer) *LogWriter {
	return &LogWriter{output: output}
}

// Reset resets the state of the log writer to produce to output to the given
// io.Writer.
func (w *LogWriter) Reset(output io.Writer) {
	w.output = output
	w.stickyErr = nil
}

// WriteRecordBatch writes a record batch to the log. The method returns
// a non-nil error if the write failed.
//
// If the error occurred while writing to the underlying io.Writer, the writer
// is broken and will always error on future calls to WriteRecordBatch until
// the program calls Reset.
func (w *LogWriter) WriteRecordBatch(batch *RecordBatchBuilder) error {
	if w.stickyErr != nil {
		return w.stickyErr
	}
	_, err := batch.Write(w.output)
	return err
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
//
// The record is consumed immediately and can be reused safely when the
// call returns.
func (w *LogRecordWriter) WriteRecord(record *RecordBuilder) error {
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
	if err := w.WriteRecordBatch(&w.batch); err != nil {
		return err
	}
	w.firstOffset += int64(w.count)
	w.count = 0
	w.batch.Reset(w.compression, w.firstOffset)
	return nil
}
