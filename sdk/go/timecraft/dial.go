//go:build wasip1

package timecraft

import (
	"context"
	"net"
	"net/http"
	"os"
	"syscall"
	"unsafe"

	_ "github.com/stealthrocket/net/http"
	"github.com/stealthrocket/net/wasip1"
	"github.com/stealthrocket/timecraft/internal/htls"
)

func init() {
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		t.DialTLSContext = DialTLS
	}
}

//go:wasmimport wasi_snapshot_preview1 sock_setsockopt
//go:noescape
func setsockopt(fd int32, level uint32, name uint32, value unsafe.Pointer, valueLen uint32) syscall.Errno

// DialTLS connects to the given network address and performs a TLS handshake
// with the host. The resulting connection transparently performs encryption.
func DialTLS(ctx context.Context, network, addr string) (net.Conn, error) {
	switch network {
	case "tcp", "tcp4", "tcp6", "unix":
	default:
		return nil, net.UnknownNetworkError(network)
	}
	hostname, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	host := []byte(hostname)
	conn, err := wasip1.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	syscallConn := conn.(syscall.Conn)
	rawConn, err := syscallConn.SyscallConn()
	if err != nil {
		conn.Close()
		return nil, &net.OpError{
			Op:     "dial",
			Net:    network,
			Source: conn.LocalAddr(),
			Addr:   conn.RemoteAddr(),
			Err:    err,
		}
	}

	var errno syscall.Errno
	err = rawConn.Control(func(fd uintptr) {
		errno = setsockopt(int32(fd), htls.Level, htls.Option, unsafe.Pointer(unsafe.SliceData(host)), uint32(len(hostname)))
	})
	if errno != 0 {
		err = os.NewSyscallError("setsockopt", errno)
	}

	if err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}
