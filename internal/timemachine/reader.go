package timemachine

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/stealthrocket/timecraft/format/logsegment"
	"github.com/stealthrocket/timecraft/format/types"
)

var (
	errMissingRuntime          = errors.New("missing runtime in log header")
	errMissingProcess          = errors.New("missing process in log header")
	errMissingProcessID        = errors.New("missing process id in log header")
	errMissingProcessImage     = errors.New("missing process image in log header")
	errMissingProcessStartTime = errors.New("missing process start time in log header")
)

const maxFrameSize = (1 * 1024 * 1024) - 4

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
func (r *LogReader) ReadLogHeader() (*Header, int64, error) {
	f, err := r.readFrameAt(0)
	if err != nil {
		return nil, 0, err
	}
	defer frameBufferPool.put(f)

	var h Header
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

func (r *LogReader) ReadRecordBatch(header *Header, byteOffset int64) (*RecordBatch, int64, error) {
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
	size := int64(len(f.data))
	if header.Compression == Uncompressed {
		size += int64(b.UncompressedSize())
	} else {
		size += int64(b.CompressedSize())
	}
	return batch, size, nil
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

// LogRecordIterator is a helper for iterating records in a log.
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
		if i.batch != nil {
			i.batch.Close()
		}
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

func (i *LogRecordIterator) Close() error {
	if i.batch != nil {
		i.batch.Close()
		i.batch = nil
	}
	return i.err
}
