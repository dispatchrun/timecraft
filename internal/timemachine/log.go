package timemachine

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

const maxFrameSize = (1 * 1024 * 1024) - 4

// LogReader instances allow programs to read the content of a record log.
//
// The LogReader type has two main methods, ReadLogHeader and ReadRecordBatch.
// ReadLogHeader should be called first to load the header needed to read
// log records. ReadRecordBatch may be called multiple times until io.EOF is
// returned to scan through the log.
type LogReader struct {
	input      io.ReaderAt
	bufferSize int

	batch      RecordBatch
	batchFrame *buffer
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

// Close closes the log reader.
func (r *LogReader) Close() error {
	releaseBuffer(&r.batchFrame, &frameBufferPool)
	r.batch.Reset(nil, nil, nil)
	return nil
}

// ReadLogHeader reads and returns the log header from r.
//
// The log header is always located at the first byte of the underlying segment.
//
// The method returns the log header that was read, along with the number of
// bytes that it spanned over. If the log header could not be read, a non-nil
// error is returned describing the reason why.
func (r *LogReader) ReadLogHeader() (*Header, int64, error) {
	f, err := r.readFrameAt(0)
	if err != nil {
		return nil, 0, err
	}
	defer frameBufferPool.put(f)

	header, err := NewHeader(f.data)
	if err != nil {
		return nil, 0, err
	}
	return header, int64(len(f.data)), nil
}

// ReadRecordBatch reads a batch at the specified offset.
//
// The RecordBatch is only valid until the next call to ReadRecordBatch.
func (r *LogReader) ReadRecordBatch(header *Header, byteOffset int64) (*RecordBatch, int64, error) {
	releaseBuffer(&r.batchFrame, &frameBufferPool)
	var err error
	r.batchFrame, err = r.readFrameAt(byteOffset)
	if err != nil {
		return nil, 0, err
	}
	recordBatchSize := int64(len(r.batchFrame.data))
	recordsReader := io.NewSectionReader(r.input, byteOffset+recordBatchSize, math.MaxInt64)
	r.batch.Reset(header, r.batchFrame.data, recordsReader)
	return &r.batch, recordBatchSize + int64(r.batch.RecordsSize()), nil
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

// LogRecordIterator is a higher level helper for iterating records in a log.
type LogRecordIterator struct {
	reader       *LogReader
	header       *Header
	batch        *RecordBatch
	record       Record
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
			return true // i.err is returned on call to Record()
		}
	}
	for i.batch == nil || !i.batch.Next() {
		var batchLength int64
		i.batch, batchLength, i.err = i.reader.ReadRecordBatch(i.header, i.readerOffset)
		if i.err != nil {
			return i.err != io.EOF
		}
		i.readerOffset += batchLength
	}
	i.record, i.err = i.batch.Record()
	return true
}

// Record returns the next record as a RecordReader.
//
// The return value is only valid when Next returns true.
func (i *LogRecordIterator) Record() (Record, error) {
	return i.record, i.err
}

// Close closes the iterator.
func (i *LogRecordIterator) Close() error {
	return i.err
}

// LogWriter supports writing log segments to an io.Writer.
//
// The type has two main methods, WriteLogHeader and WriteRecordBatch.
// The former must be called first and only once to write the log header and
// initialize the state of the writer, zero or more record batches may then
// be written to the log after that.
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
//
// WriteLogHeader should be called again after resetting the writer.
func (w *LogWriter) Reset(output io.Writer) {
	w.output = output
	w.stickyErr = nil
}

// WriteLogHeader writes the log header. The method must be called before any
// records are written to the log via calls to WriteRecordBatch.
func (w *LogWriter) WriteLogHeader(header HeaderBuilder) error {
	if w.stickyErr != nil {
		return w.stickyErr
	}
	if _, err := w.output.Write(header.Bytes()); err != nil {
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
