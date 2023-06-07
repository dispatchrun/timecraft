package timemachine

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/stealthrocket/timecraft/internal/buffer"
	"github.com/stealthrocket/timecraft/internal/stream"
)

const maxFrameSize = (1 * 1024 * 1024) - 4

var frameBufferPool buffer.Pool

// LogReader instances allow programs to read the content of a record log.
type LogReader struct {
	input            io.ReadSeeker
	nextByteOffset   int64
	nextRecordOffset int64
	bufferSize       int64
	startTime        time.Time
	batch            RecordBatch
	batchFrame       *buffer.Buffer
}

// NewLogReader construct a new log reader consuming input from the given
// io.Reader.
func NewLogReader(input io.ReadSeeker, startTime time.Time) *LogReader {
	return NewLogReaderSize(input, startTime, buffer.DefaultSize)
}

// NewLogReaderSize is like NewLogReader but it allows the program to configure
// the read buffer size.
func NewLogReaderSize(input io.ReadSeeker, startTime time.Time, bufferSize int64) *LogReader {
	return &LogReader{
		startTime:  startTime,
		input:      input,
		bufferSize: buffer.Align(bufferSize, buffer.DefaultSize),
	}
}

// Close closes the log reader.
func (r *LogReader) Close() error {
	buffer.Release(&r.batchFrame, &frameBufferPool)
	r.batch.Reset(r.startTime, nil, nil)
	r.nextByteOffset = 0
	r.nextRecordOffset = 0
	return nil
}

// Seek positions r on the first record batch which contains the given record
// offset.
//
// The whence value defines how to interpret the offset (see io.Seeker).
func (r *LogReader) Seek(offset int64, whence int) (int64, error) {
	nextRecordOffset, err := stream.Seek(r.nextRecordOffset, math.MaxInt64, offset, whence)
	if err != nil {
		return -1, err
	}
	// Seek was called with an offset which is before the current position,
	// we have to rewind to the start and let the loop below seek the first
	// batch which includes the record offset.
	if nextRecordOffset < r.batch.NextOffset() {
		_, err := r.input.Seek(0, io.SeekStart)
		if err != nil {
			return -1, err
		}
		r.batch.Reset(r.startTime, nil, nil)
		r.nextByteOffset = 0
	}
	r.nextRecordOffset = nextRecordOffset
	return nextRecordOffset, nil
}

// ReadRecordBatch reads the next record batch.
//
// The RecordBatch is only valid until the next call to ReadRecordBatch or Seek.
func (r *LogReader) ReadRecordBatch() (*RecordBatch, error) {
	for {
		// There may be data left that was not consumed from the previous batch
		// returned by ReadRecordBatch (e.g. if the program read the metadata
		// and decided it wasn't interested in the batch). We know the byte
		// offset of the current record batch
		if r.batch.reader.N > 0 {
			_, err := r.input.Seek(r.nextByteOffset, io.SeekStart)
			if err != nil {
				return nil, err
			}
			r.batch.reader.R = nil
			r.batch.reader.N = 0
		}

		buffer.Release(&r.batchFrame, &frameBufferPool)

		var err error
		r.batchFrame, err = r.readFrame()
		if err != nil {
			return nil, err
		}

		r.batch.Reset(r.startTime, r.batchFrame.Data, r.input)
		r.nextByteOffset += r.batchFrame.Size()
		r.nextByteOffset += r.batch.CompressedSize()

		if nextRecordOffset := r.batch.NextOffset(); nextRecordOffset >= r.nextRecordOffset {
			r.nextRecordOffset = nextRecordOffset
			return &r.batch, nil
		}
	}
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
		newFrame := buffer.New(int64(byteLength))
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

var (
	_ io.Closer = (*LogReader)(nil)
	_ io.Seeker = (*LogReader)(nil)
)

// LogRecordReader wraps a LogReader to help with reading individual records
// in order.
//
// The reader exposes an iterator like interface. Callers should call Next to
// determine whether another record is available. If so, it can be retrieved
// via the Record method.
type LogRecordReader struct {
	reader *LogReader
	batch  *RecordBatch
	offset int64
	seekTo int64
}

// NewLogRecordReader creates a log record iterator.
func NewLogRecordReader(r *LogReader) *LogRecordReader {
	return &LogRecordReader{reader: r}
}

// Read reads records from r.
//
// The record values share memory buffer with the reader, they remain valid
// until the next call to Read or Seek.
func (r *LogRecordReader) Read(records []Record) (int, error) {
	for {
		if r.batch == nil {
			b, err := r.reader.ReadRecordBatch()
			if err != nil {
				return 0, err
			}
			if b.NextOffset() < r.seekTo {
				continue
			}
			r.batch = b
			r.offset = b.FirstOffset()
		}

		n, err := r.batch.Read(records)
		offset := r.offset
		r.offset += int64(n)

		if offset < r.seekTo {
			if skip := r.seekTo - offset; skip < int64(n) {
				n = copy(records, records[skip:])
			} else {
				n = 0
			}
		}

		if n > 0 {
			return n, nil
		}
		if err != io.EOF {
			return 0, err
		}

		r.batch = nil
	}
}

// Seek positions the reader on the record at the given offset.
//
// The whence value defines how to interpret the offset (see io.Seeker).
func (r *LogRecordReader) Seek(offset int64, whence int) (int64, error) {
	seekTo, err := r.reader.Seek(offset, whence)
	if err != nil {
		return -1, err
	}
	if seekTo < r.offset {
		r.batch, r.offset = nil, 0
	} else if r.batch != nil {
		if nextOffset := r.batch.NextOffset(); seekTo >= nextOffset {
			r.batch, r.offset = nil, nextOffset
		}
	}
	r.seekTo = seekTo
	return seekTo, nil
}

var (
	_ stream.ReadSeeker[Record] = (*LogRecordReader)(nil)
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
