package timemachine

import (
	"fmt"
	"io"
	"slices"
	"time"

	flatbuffers "github.com/google/flatbuffers/go"
	"github.com/stealthrocket/timecraft/format/logsegment"
	"github.com/stealthrocket/timecraft/internal/stream"
)

// RecordBatch is a read-only batch of records read from a log segment.
//
// The records themselves are compressed and stored separately. To support
// use cases where the user may want to read batch metadata in order to skip
// the processing of records, the record batch is structured such that records
// are read and decompressed lazily.
type RecordBatch struct {
	// Reader for the records data section adjacent to the record batch. When
	// the records are accessed, they're lazily read into the records buffer.
	reader    io.LimitedReader
	startTime time.Time
	batch     logsegment.RecordBatch
	// When reading records from the batch, this holds the current offset into
	// the records buffer.
	offset uint32
	index  uint32
	// These buffers are retained by the record batch to be reused when Reset
	// is called.
	compressed   []byte
	uncompressed []byte
}

// Reset resets the record batch.
func (b *RecordBatch) Reset(startTime time.Time, buf []byte, reader io.Reader) {
	if len(buf) > 0 {
		b.batch = *logsegment.GetSizePrefixedRootAsRecordBatch(buf, 0)
		b.reader.R = reader
		b.reader.N = b.CompressedSize()
	} else {
		b.batch = logsegment.RecordBatch{}
		b.reader.R = nil
		b.reader.N = 0
	}
	b.startTime = startTime
	b.offset = 0
	b.index = 0
	b.compressed = b.compressed[:0]
	b.uncompressed = b.uncompressed[:0]
}

// Compression returns the compression algorithm used to encode the record
// batch data section.
func (b *RecordBatch) Compression() Compression {
	return b.batch.Compression()
}

// FirstOffset returns the logical offset of the first record in the batch.
func (b *RecordBatch) FirstOffset() int64 {
	return b.batch.FirstOffset()
}

// NextOffset returns the offset of the first record after this batch.
func (b *RecordBatch) NextOffset() int64 {
	return b.batch.FirstOffset() + int64(b.NumRecords())
}

// FirstTimestamp returns the time of the first record in the batch.
func (b *RecordBatch) FirstTimestamp() time.Time {
	return b.startTime.Add(time.Duration(b.batch.FirstTimestamp()))
}

// LastTimestamp returns the time of the last record in the batch.
func (b *RecordBatch) LastTimestamp() time.Time {
	return b.startTime.Add(time.Duration(b.batch.LastTimestamp()))
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

// Read reads records from the batch.
//
// The record values share memory buffers with the record batch, they remain
// valid until the next call to ReadRecordBatch on the parent LogReader.
func (b *RecordBatch) Read(records []Record) (int, error) {
	batch, err := b.readRecords()
	if err != nil {
		return 0, err
	}
	for n := range records {
		if b.offset == uint32(len(batch)) {
			return n, io.EOF
		}
		if b.offset+4 > uint32(len(batch)) {
			return n, fmt.Errorf("cannot read record at offset %d as records buffer is length %d: %w", b.offset, len(batch), io.ErrUnexpectedEOF)
		}
		size := flatbuffers.GetSizePrefix(batch, flatbuffers.UOffsetT(b.offset))
		if b.offset+size < b.offset || b.offset+size > uint32(len(batch)) {
			return n, fmt.Errorf("cannot read record at [%d:%d+%d] as records buffer is length %d: %w", b.offset, b.offset, size, len(batch), io.ErrUnexpectedEOF)
		}
		i := 4 + b.offset
		j := 4 + b.offset + size
		r := logsegment.GetRootAsRecord(batch[i:j:j], 0)
		records[n] = Record{
			Offset:       b.FirstOffset() + int64(b.index),
			Time:         b.startTime.Add(time.Duration(r.Timestamp())),
			FunctionID:   int(r.FunctionId()),
			FunctionCall: r.FunctionCallBytes(),
		}
		b.offset += size + 4
		b.index++
	}
	return len(records), nil
}

func (b *RecordBatch) readRecords() ([]byte, error) {
	if len(b.uncompressed) > 0 {
		return b.uncompressed, nil
	}

	compression := b.Compression()
	data := b.compressed
	size := b.CompressedSize()

	if compression == Uncompressed {
		data = b.uncompressed
		size = b.UncompressedSize()
		defer func() { b.uncompressed = data }()
	} else {
		defer func() { b.compressed = data }()
	}

	data = slices.Grow(data[:0], int(size))
	data = data[:size]

	_, err := io.ReadFull(&b.reader, data)
	if err != nil {
		return nil, err
	}

	if c := checksum(data); c != b.batch.Checksum() {
		return nil, fmt.Errorf("bad record data: expect checksum %#x, got %#x", b.batch.Checksum(), c)
	}

	if compression == Uncompressed {
		return data, nil
	}

	uncompressedSize := int(b.UncompressedSize())
	b.uncompressed = slices.Grow(b.uncompressed[:0], uncompressedSize)
	b.uncompressed = b.uncompressed[:uncompressedSize]
	return decompress(b.uncompressed, data, compression)
}

var (
	_ stream.Reader[Record] = (*RecordBatch)(nil)
)

// RecordBatchBuilder is a builder for record batches.
type RecordBatchBuilder struct {
	builder        *flatbuffers.Builder
	compression    Compression
	firstOffset    int64
	firstTimestamp int64
	lastTimestamp  int64
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
	b.lastTimestamp = 0
	b.finished = false
	b.concatenated = false
	b.recordCount = 0
}

// AddRecord adds a record to the batch.
//
// The record is consumed immediately and can be reused safely when the
// call returns.
func (b *RecordBatchBuilder) AddRecord(record *RecordBuilder) {
	if b.finished {
		panic("builder must be reset before records can be added")
	}
	b.uncompressed = append(b.uncompressed, record.Bytes()...)
	if b.recordCount == 0 {
		b.firstTimestamp = record.timestamp
	}
	b.lastTimestamp = record.timestamp
	b.recordCount++
}

// Bytes returns the serialized representation of the record batch.
//
// Since the batch is made up of two components – the batch metadata
// and then the compressed records – additional buffering is required
// here to merge the two together. If efficiency is required, Write
// should be used instead.
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
		b.builder = flatbuffers.NewBuilder(defaultBufferSize)
	}
	b.builder.Reset()

	b.records = b.uncompressed
	uncompressedSize := uint32(len(b.uncompressed))
	compressedSize := uncompressedSize

	if b.compression != Uncompressed {
		b.compressed = compress(b.compressed[:cap(b.compressed)], b.uncompressed, b.compression)
		b.records = b.compressed
		compressedSize = uint32(len(b.compressed))
	}

	logsegment.RecordBatchStart(b.builder)
	logsegment.RecordBatchAddFirstOffset(b.builder, b.firstOffset)
	logsegment.RecordBatchAddFirstTimestamp(b.builder, b.firstTimestamp)
	logsegment.RecordBatchAddLastTimestamp(b.builder, b.lastTimestamp)
	logsegment.RecordBatchAddCompressedSize(b.builder, compressedSize)
	logsegment.RecordBatchAddUncompressedSize(b.builder, uncompressedSize)
	logsegment.RecordBatchAddChecksum(b.builder, checksum(b.records))
	logsegment.RecordBatchAddNumRecords(b.builder, b.recordCount)
	logsegment.RecordBatchAddCompression(b.builder, b.compression)
	b.builder.FinishSizePrefixed(logsegment.RecordBatchEnd(b.builder))
}
