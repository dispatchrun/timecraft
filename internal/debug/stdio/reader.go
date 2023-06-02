package stdio

import (
	"bytes"
	"io"
	"math"
	"time"

	"github.com/stealthrocket/timecraft/internal/stream"
	"github.com/stealthrocket/timecraft/internal/timemachine"
	"github.com/stealthrocket/timecraft/internal/timemachine/wasicall"
	"github.com/stealthrocket/wasi-go"
)

type Reader struct {
	Records   stream.Reader[timemachine.Record]
	StartTime time.Time
	Stdout    int
	Stderr    int

	buffer  bytes.Buffer
	stdout  wasi.FD
	stderr  wasi.FD
	iovecs  []wasi.IOVec
	codec   wasicall.Codec
	records [100]timemachine.Record
}

const noneFD = ^wasi.FD(0)

func makeFD(fd int) wasi.FD {
	if fd < 0 || fd > math.MaxInt32 {
		return noneFD
	}
	return wasi.FD(fd)
}

func (r *Reader) Read(b []byte) (n int, err error) {
	if r.stdout != noneFD {
		r.stdout = makeFD(r.Stdout)
	}
	if r.stderr != noneFD {
		r.stderr = makeFD(r.Stderr)
	}

	for {
		if r.buffer.Len() > 0 {
			rn, _ := r.buffer.Read(b[n:])
			n += rn
		}
		if n == len(b) || err != nil {
			return n, err
		}
		if r.stdout == noneFD && r.stderr == noneFD {
			return n, io.EOF
		}
		var rn int
		rn, err = r.Records.Read(r.records[:])

		for _, record := range r.records[:rn] {
			switch wasicall.SyscallID(record.FunctionID) {
			case wasicall.FDClose:
				fd, _, err := r.codec.DecodeFDClose(record.FunctionCall)
				if err != nil {
					return n, err
				}
				switch fd {
				case r.stdout:
					r.stdout = noneFD
				case r.stderr:
					r.stderr = noneFD
				}

			case wasicall.FDRenumber:
				from, to, errno, err := r.codec.DecodeFDRenumber(record.FunctionCall)
				if err != nil {
					return n, err
				}
				if errno != wasi.ESUCCESS {
					continue
				}
				switch from {
				case r.stdout:
					r.stdout = to
				case r.stderr:
					r.stderr = to
				}

			case wasicall.FDWrite:
				if record.Time.Before(r.StartTime) {
					continue
				}
				fd, iovecs, size, _, err := r.codec.DecodeFDWrite(record.FunctionCall, r.iovecs[:0])
				if err != nil {
					return n, err
				}
				switch fd {
				case r.stdout, r.stderr:
					for _, iov := range iovecs {
						iovLen := wasi.Size(len(iov))
						if iovLen > size {
							iovLen = size
						}
						size -= iovLen
						r.buffer.Write(iov[:iovLen])
					}
				}
			}
		}
	}
}
