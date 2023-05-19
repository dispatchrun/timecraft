package timemachine

import (
	"fmt"
	"io"
	"sync"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
)

// RecordBatch is a read-only batch of records read from a log segment.
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
	if b.offset+4 < b.offset || b.offset+4 > uint32(len(records)) {
		b.err = fmt.Errorf("cannot read record at offset %d as records buffer is length %d", b.offset, len(records))
		return true
	}
	size := flatbuffers.GetSizePrefix(records, flatbuffers.UOffsetT(b.offset))
	if b.offset+size < b.offset || b.offset+size > uint32(len(records)) {
		b.err = fmt.Errorf("cannot read record at [%d:%d+%d] as records buffer is length %d", b.offset, b.offset, size, len(records))
		return true
	}
	b.record = MakeRecord(b.header.Process.StartTime, b.header.Runtime.Functions, records[b.offset:b.offset+4+size])
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

// RecordBatchBuilder is a builder for record batches.
type RecordBatchBuilder struct {
	builder        *flatbuffers.Builder
	compression    Compression
	firstOffset    int64
	firstTimestamp int64
	recordCount    uint32
	uncompressed   []byte
	compressed     []byte
	records        []byte
	result         []byte
	finished       bool
	concatenated   bool
}

// Reset resets the builder.
func (b *RecordBatchBuilder) Reset(compression Compression, firstOffset int64) {
	if b.builder == nil {
		b.builder = flatbuffers.NewBuilder(defaultBufferSize)
	} else {
		b.builder.Reset()
	}
	b.compression = compression
	b.uncompressed = b.uncompressed[:0]
	b.compressed = b.compressed[:0]
	b.result = b.result[:0]
	b.records = nil
	b.firstOffset = firstOffset
	b.firstTimestamp = 0
	b.finished = false
	b.concatenated = false
	b.recordCount = 0
}

// AddRecord adds a record to the batch.
func (b *RecordBatchBuilder) AddRecord(record RecordBuilder) {
	if b.finished {
		panic("builder must be reset before records can be added")
	}
	b.uncompressed = append(b.uncompressed, record.Bytes()...)
	if b.recordCount == 0 {
		b.firstTimestamp = record.timestamp
	}
	b.recordCount++
}

// Bytes returns the serialized representation of the record batch.
func (b *RecordBatchBuilder) Bytes() []byte {
	if !b.finished {
		b.build()
		b.finished = true
	}
	if !b.concatenated {
		b.result = append(b.result, b.builder.FinishedBytes()...)
		b.result = append(b.result, b.records...)
		b.concatenated = true
	}
	return b.result
}

// Write writes the serialized representation of the record batch
// to the specified writer.
func (b *RecordBatchBuilder) Write(w io.Writer) (int, error) {
	if !b.finished {
		b.build()
		b.finished = true
	}
	n1, err := w.Write(b.builder.FinishedBytes())
	if err != nil {
		return n1, err
	}
	n2, err := w.Write(b.records)
	return n1 + n2, err
}

func (b *RecordBatchBuilder) build() {
	if b.builder == nil {
		panic("builder is not initialized")
	}

	b.records = b.uncompressed
	if b.compression != Uncompressed {
		b.compressed = compress(b.compressed[:cap(b.compressed)], b.uncompressed, b.compression)
		b.records = b.compressed
	}

	logsegment.RecordBatchStart(b.builder)
	logsegment.RecordBatchAddFirstOffset(b.builder, b.firstOffset)
	logsegment.RecordBatchAddFirstTimestamp(b.builder, b.firstTimestamp)
	logsegment.RecordBatchAddCompressedSize(b.builder, uint32(len(b.compressed)))
	logsegment.RecordBatchAddUncompressedSize(b.builder, uint32(len(b.uncompressed)))
	logsegment.RecordBatchAddChecksum(b.builder, checksum(b.records))
	logsegment.RecordBatchAddNumRecords(b.builder, b.recordCount)
	logsegment.FinishSizePrefixedRecordBatchBuffer(b.builder, logsegment.RecordBatchEnd(b.builder))
}
