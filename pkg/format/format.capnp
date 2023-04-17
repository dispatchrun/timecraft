using Go = import "/go.capnp";
@0xc919885706816759;
$Go.package("format");
$Go.import("github.com/stealthrocket/timecraft/pkg/format");

enum Compression {
    # Compression is the enumeration representing the supported compression
    # algorithms for data sections of log snapshots.
    uncompressed @0;
    snappy @1;
    zstd @2;
}

enum MemoryAccessType {
    read @0;
    write @1;
}

struct Hash {
    # Hash represents a OCI hash which pairs an algorithm name to a digest.
    # The digest length depends on the algorithm in use (e.g. 32 bytes for
    # "sha256").
    algorithm @0 :Text;
    digest @1 :Text;
}

struct Function {
    # Function represents a function from a host module.
    module @0 :Text;
    name @1 :Text;
}

struct Runtime {
    runtime @0 :Text;
    version @1 :Text;
    functions @2 :List(Function);
}

struct Process {
    id @0 :Hash;
    image @1 :Hash;
    unixStartTime @2 :Int64;
    arguments @3 :List(Text);
    environment @4 :List(Text);
    parentProcessId @5 :Hash;
    parentForkOffset @6 :Int64;
}

struct LogHeader {
    runtime @0 :Runtime;
    process @1 :Process;
    segment @2 :UInt32 $Go.name("SegmentNumber");
    compression @3 :Compression;
}

struct RecordBatch {
    firstOffset @0 :Int64;
    compressedSize @1 :UInt32;
    uncompressedSize @2 :UInt32;
    checksum @3 :UInt32;
    records @4 :List(Record);
}

struct Record {
    timestamp @0 :Int64;
    function @1 :UInt32;
    params @2 :List(UInt64);
    results @3 :List(UInt64);
    offset @4 :UInt32;
    length @5 :UInt32;
    memoryAccess @6 :List(MemoryAccess);
}

struct MemoryAccess {
    memoryOffset @0 :UInt32;
    recordOffset @1 :UInt32;
    length @2 :UInt32;
    access @3 :MemoryAccessType;
}
