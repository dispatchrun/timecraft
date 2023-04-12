// Code generated by the FlatBuffers compiler. DO NOT EDIT.

package snapshot

import (
	"strconv"
	flatbuffers "github.com/google/flatbuffers/go"
)

type Compression uint32

const (
	CompressionNone   Compression = 0
	CompressionSnappy Compression = 1
	CompressionZstd   Compression = 2
)

var EnumNamesCompression = map[Compression]string{
	CompressionNone:   "None",
	CompressionSnappy: "Snappy",
	CompressionZstd:   "Zstd",
}

var EnumValuesCompression = map[string]Compression{
	"None":   CompressionNone,
	"Snappy": CompressionSnappy,
	"Zstd":   CompressionZstd,
}

func (v Compression) String() string {
	if s, ok := EnumNamesCompression[v]; ok {
		return s
	}
	return "Compression(" + strconv.FormatInt(int64(v), 10) + ")"
}

type Hash struct {
	_tab flatbuffers.Table
}

func GetRootAsHash(buf []byte, offset flatbuffers.UOffsetT) *Hash {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &Hash{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsHash(buf []byte, offset flatbuffers.UOffsetT) *Hash {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &Hash{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *Hash) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *Hash) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *Hash) Algorithm() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *Hash) Digest(j int) byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.GetByte(a + flatbuffers.UOffsetT(j*1))
	}
	return 0
}

func (rcv *Hash) DigestLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *Hash) DigestBytes() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func (rcv *Hash) MutateDigest(j int, n byte) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		a := rcv._tab.Vector(o)
		return rcv._tab.MutateByte(a+flatbuffers.UOffsetT(j*1), n)
	}
	return false
}

func HashStart(builder *flatbuffers.Builder) {
	builder.StartObject(2)
}
func HashAddAlgorithm(builder *flatbuffers.Builder, algorithm flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(algorithm), 0)
}
func HashAddDigest(builder *flatbuffers.Builder, digest flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(1, flatbuffers.UOffsetT(digest), 0)
}
func HashStartDigestVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(1, numElems, 1)
}
func HashEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
type RecordRange struct {
	_tab flatbuffers.Table
}

func GetRootAsRecordRange(buf []byte, offset flatbuffers.UOffsetT) *RecordRange {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &RecordRange{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsRecordRange(buf []byte, offset flatbuffers.UOffsetT) *RecordRange {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &RecordRange{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *RecordRange) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *RecordRange) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *RecordRange) ProcessId(obj *Hash) *Hash {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(Hash)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

func (rcv *RecordRange) StartTimestamp() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *RecordRange) MutateStartTimestamp(n int64) bool {
	return rcv._tab.MutateInt64Slot(6, n)
}

func (rcv *RecordRange) StartOffset() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *RecordRange) MutateStartOffset(n int64) bool {
	return rcv._tab.MutateInt64Slot(8, n)
}

func (rcv *RecordRange) NumRecords() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(10))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *RecordRange) MutateNumRecords(n int64) bool {
	return rcv._tab.MutateInt64Slot(10, n)
}

func (rcv *RecordRange) CompressedSize() uint32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(12))
	if o != 0 {
		return rcv._tab.GetUint32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *RecordRange) MutateCompressedSize(n uint32) bool {
	return rcv._tab.MutateUint32Slot(12, n)
}

func (rcv *RecordRange) UncompressedSize() uint32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(14))
	if o != 0 {
		return rcv._tab.GetUint32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *RecordRange) MutateUncompressedSize(n uint32) bool {
	return rcv._tab.MutateUint32Slot(14, n)
}

func (rcv *RecordRange) Compression() Compression {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(16))
	if o != 0 {
		return Compression(rcv._tab.GetUint32(o + rcv._tab.Pos))
	}
	return 0
}

func (rcv *RecordRange) MutateCompression(n Compression) bool {
	return rcv._tab.MutateUint32Slot(16, uint32(n))
}

func (rcv *RecordRange) Checksum() uint32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(18))
	if o != 0 {
		return rcv._tab.GetUint32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *RecordRange) MutateChecksum(n uint32) bool {
	return rcv._tab.MutateUint32Slot(18, n)
}

func (rcv *RecordRange) OciLayer(obj *Hash) *Hash {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(20))
	if o != 0 {
		x := rcv._tab.Indirect(o + rcv._tab.Pos)
		if obj == nil {
			obj = new(Hash)
		}
		obj.Init(rcv._tab.Bytes, x)
		return obj
	}
	return nil
}

func (rcv *RecordRange) OpenFiles(obj *OpenFile, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(22))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		x = rcv._tab.Indirect(x)
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *RecordRange) OpenFilesLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(22))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *RecordRange) OpenHandles(obj *OpenHandle, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(24))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		x = rcv._tab.Indirect(x)
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *RecordRange) OpenHandlesLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(24))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func (rcv *RecordRange) MemoryWrites(obj *MemoryPage, j int) bool {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(26))
	if o != 0 {
		x := rcv._tab.Vector(o)
		x += flatbuffers.UOffsetT(j) * 4
		obj.Init(rcv._tab.Bytes, x)
		return true
	}
	return false
}

func (rcv *RecordRange) MemoryWritesLength() int {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(26))
	if o != 0 {
		return rcv._tab.VectorLen(o)
	}
	return 0
}

func RecordRangeStart(builder *flatbuffers.Builder) {
	builder.StartObject(12)
}
func RecordRangeAddProcessId(builder *flatbuffers.Builder, processId flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(0, flatbuffers.UOffsetT(processId), 0)
}
func RecordRangeAddStartTimestamp(builder *flatbuffers.Builder, startTimestamp int64) {
	builder.PrependInt64Slot(1, startTimestamp, 0)
}
func RecordRangeAddStartOffset(builder *flatbuffers.Builder, startOffset int64) {
	builder.PrependInt64Slot(2, startOffset, 0)
}
func RecordRangeAddNumRecords(builder *flatbuffers.Builder, numRecords int64) {
	builder.PrependInt64Slot(3, numRecords, 0)
}
func RecordRangeAddCompressedSize(builder *flatbuffers.Builder, compressedSize uint32) {
	builder.PrependUint32Slot(4, compressedSize, 0)
}
func RecordRangeAddUncompressedSize(builder *flatbuffers.Builder, uncompressedSize uint32) {
	builder.PrependUint32Slot(5, uncompressedSize, 0)
}
func RecordRangeAddCompression(builder *flatbuffers.Builder, compression Compression) {
	builder.PrependUint32Slot(6, uint32(compression), 0)
}
func RecordRangeAddChecksum(builder *flatbuffers.Builder, checksum uint32) {
	builder.PrependUint32Slot(7, checksum, 0)
}
func RecordRangeAddOciLayer(builder *flatbuffers.Builder, ociLayer flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(8, flatbuffers.UOffsetT(ociLayer), 0)
}
func RecordRangeAddOpenFiles(builder *flatbuffers.Builder, openFiles flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(9, flatbuffers.UOffsetT(openFiles), 0)
}
func RecordRangeStartOpenFilesVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func RecordRangeAddOpenHandles(builder *flatbuffers.Builder, openHandles flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(10, flatbuffers.UOffsetT(openHandles), 0)
}
func RecordRangeStartOpenHandlesVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 4)
}
func RecordRangeAddMemoryWrites(builder *flatbuffers.Builder, memoryWrites flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(11, flatbuffers.UOffsetT(memoryWrites), 0)
}
func RecordRangeStartMemoryWritesVector(builder *flatbuffers.Builder, numElems int) flatbuffers.UOffsetT {
	return builder.StartVector(4, numElems, 2)
}
func RecordRangeEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
type OpenFile struct {
	_tab flatbuffers.Table
}

func GetRootAsOpenFile(buf []byte, offset flatbuffers.UOffsetT) *OpenFile {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &OpenFile{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsOpenFile(buf []byte, offset flatbuffers.UOffsetT) *OpenFile {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &OpenFile{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *OpenFile) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *OpenFile) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *OpenFile) Fd() int32 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetInt32(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *OpenFile) MutateFd(n int32) bool {
	return rcv._tab.MutateInt32Slot(4, n)
}

func (rcv *OpenFile) Seek() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(6))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *OpenFile) MutateSeek(n int64) bool {
	return rcv._tab.MutateInt64Slot(6, n)
}

func (rcv *OpenFile) Path() []byte {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(8))
	if o != 0 {
		return rcv._tab.ByteVector(o + rcv._tab.Pos)
	}
	return nil
}

func OpenFileStart(builder *flatbuffers.Builder) {
	builder.StartObject(3)
}
func OpenFileAddFd(builder *flatbuffers.Builder, fd int32) {
	builder.PrependInt32Slot(0, fd, 0)
}
func OpenFileAddSeek(builder *flatbuffers.Builder, seek int64) {
	builder.PrependInt64Slot(1, seek, 0)
}
func OpenFileAddPath(builder *flatbuffers.Builder, path flatbuffers.UOffsetT) {
	builder.PrependUOffsetTSlot(2, flatbuffers.UOffsetT(path), 0)
}
func OpenFileEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
type OpenHandle struct {
	_tab flatbuffers.Table
}

func GetRootAsOpenHandle(buf []byte, offset flatbuffers.UOffsetT) *OpenHandle {
	n := flatbuffers.GetUOffsetT(buf[offset:])
	x := &OpenHandle{}
	x.Init(buf, n+offset)
	return x
}

func GetSizePrefixedRootAsOpenHandle(buf []byte, offset flatbuffers.UOffsetT) *OpenHandle {
	n := flatbuffers.GetUOffsetT(buf[offset+flatbuffers.SizeUint32:])
	x := &OpenHandle{}
	x.Init(buf, n+offset+flatbuffers.SizeUint32)
	return x
}

func (rcv *OpenHandle) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *OpenHandle) Table() flatbuffers.Table {
	return rcv._tab
}

func (rcv *OpenHandle) Handle() int64 {
	o := flatbuffers.UOffsetT(rcv._tab.Offset(4))
	if o != 0 {
		return rcv._tab.GetInt64(o + rcv._tab.Pos)
	}
	return 0
}

func (rcv *OpenHandle) MutateHandle(n int64) bool {
	return rcv._tab.MutateInt64Slot(4, n)
}

func OpenHandleStart(builder *flatbuffers.Builder) {
	builder.StartObject(1)
}
func OpenHandleAddHandle(builder *flatbuffers.Builder, handle int64) {
	builder.PrependInt64Slot(0, handle, 0)
}
func OpenHandleEnd(builder *flatbuffers.Builder) flatbuffers.UOffsetT {
	return builder.EndObject()
}
type MemoryPage struct {
	_tab flatbuffers.Struct
}

func (rcv *MemoryPage) Init(buf []byte, i flatbuffers.UOffsetT) {
	rcv._tab.Bytes = buf
	rcv._tab.Pos = i
}

func (rcv *MemoryPage) Table() flatbuffers.Table {
	return rcv._tab.Table
}

func (rcv *MemoryPage) MemoryPageNumber() uint16 {
	return rcv._tab.GetUint16(rcv._tab.Pos + flatbuffers.UOffsetT(0))
}
func (rcv *MemoryPage) MutateMemoryPageNumber(n uint16) bool {
	return rcv._tab.MutateUint16(rcv._tab.Pos+flatbuffers.UOffsetT(0), n)
}

func (rcv *MemoryPage) RecordPageNumber() uint16 {
	return rcv._tab.GetUint16(rcv._tab.Pos + flatbuffers.UOffsetT(2))
}
func (rcv *MemoryPage) MutateRecordPageNumber(n uint16) bool {
	return rcv._tab.MutateUint16(rcv._tab.Pos+flatbuffers.UOffsetT(2), n)
}

func CreateMemoryPage(builder *flatbuffers.Builder, memoryPageNumber uint16, recordPageNumber uint16) flatbuffers.UOffsetT {
	builder.Prep(2, 4)
	builder.PrependUint16(recordPageNumber)
	builder.PrependUint16(memoryPageNumber)
	return builder.Offset()
}
