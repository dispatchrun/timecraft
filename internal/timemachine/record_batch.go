package timemachine

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/stealthrocket/timecraft/format/logsegment"
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
	header *Header

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
	record Record
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
		record, err := b.Record()
		if err != nil {
			return nil, err
		}
		records = append(records, record)
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

	if i := record.FunctionId(); i >= uint32(len(b.header.Runtime.Functions)) {
		b.err = fmt.Errorf("record points to invalid function %d", i)
		return true
	}
	b.record = Record{
		header: b.header,
		record: *record,
		FunctionCall: FunctionCall{
			function: &b.header.Runtime.Functions[record.FunctionId()],
			call:     *functionCall,
		},
	}
	b.offset += size + 4
	return true
}

// Rewind rewinds the batch in order to read records again.
func (b *RecordBatch) Rewind() {
	b.offset = 0
}

// Record returns the next record in the batch.
func (b *RecordBatch) Record() (Record, error) {
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
