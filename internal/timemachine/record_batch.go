package timemachine

import (
	"fmt"
	"io"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
)

// RecordBatch is a read-only batch of records read from a log segment.
type RecordBatch struct {
	// Reader for the records data section adjacent to the record batch. When
	// the records are accessed, they're read into the records buffer.
	recordsReader io.Reader
	records       *buffer

	batch logsegment.RecordBatch

	// Capture of the log header for the segment that the record batch was read
	// from.
	header *Header

	// If an error occurs while reading, it is captured in `err` and all reads of
	// the records will observe the error.
	err error

	// When using the batch as a record iterator, this holds the current offset
	// into the records.
	offset uint32
	record Record
}

// MakeRecordBatch creates a record batch from the specified buffer.
func MakeRecordBatch(header *Header, buf []byte, reader io.Reader) (rb RecordBatch) {
	rb.Reset(header, buf, reader)
	return
}

// Reset resets the record batch.
func (b *RecordBatch) Reset(header *Header, buf []byte, reader io.Reader) {
	if b.records != nil {
		recordsBufferPool := &compressedBufferPool
		if b.header.Compression == Uncompressed {
			recordsBufferPool = &uncompressedBufferPool
		}
		releaseBuffer(&b.records, recordsBufferPool)
	}
	b.recordsReader = reader
	b.records = nil
	b.header = header
	if len(buf) > 0 {
		b.batch = *logsegment.GetSizePrefixedRootAsRecordBatch(buf, 0)
	} else {
		b.batch = logsegment.RecordBatch{}
	}
	b.err = nil
	b.offset = 0
	b.record = Record{}
}

// RecordsSize is the size of the adjacent record data.
func (b *RecordBatch) RecordsSize() (size int) {
	if b.Compression() == Uncompressed {
		size = int(b.UncompressedSize())
	} else {
		size = int(b.CompressedSize())
	}
	return
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
	if b.err != nil {
		return true // the error is returned on the Record() call
	}
	records, err := b.readRecords()
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

func (b *RecordBatch) readRecords() ([]byte, error) {
	if b.records != nil {
		return b.records.data, nil
	}

	recordsBufferPool := &compressedBufferPool
	if b.header.Compression == Uncompressed {
		recordsBufferPool = &uncompressedBufferPool
	}
	recordsBuffer := recordsBufferPool.get(b.RecordsSize())

	_, err := io.ReadFull(b.recordsReader, recordsBuffer.data)
	if err != nil {
		recordsBufferPool.put(recordsBuffer)
		return nil, err
	}

	if c := checksum(recordsBuffer.data); c != b.batch.Checksum() {
		return nil, fmt.Errorf("bad record data: expect checksum %#x, got %#x", b.batch.Checksum(), c)
	}

	if b.header.Compression == Uncompressed {
		b.records = recordsBuffer
		return recordsBuffer.data, nil
	}
	defer recordsBufferPool.put(recordsBuffer)

	b.records = uncompressedBufferPool.get(int(b.UncompressedSize()))
	return decompress(b.records.data, recordsBuffer.data, b.header.Compression)
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
