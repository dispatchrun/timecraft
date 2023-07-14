package network

import (
	"io"
	"net"
	"sync"
)

type closeReader interface {
	CloseRead() error
}

type closeWriter interface {
	CloseWrite() error
}

var (
	_ closeReader = (*net.UnixConn)(nil)
	_ closeWriter = (*net.UnixConn)(nil)
)

func closeRead(conn io.Closer) error {
	switch c := conn.(type) {
	case closeReader:
		return c.CloseRead()
	default:
		return c.Close()
	}
}

func closeWrite(conn io.Closer) error {
	switch c := conn.(type) {
	case closeWriter:
		return c.CloseWrite()
	default:
		return c.Close()
	}
}

func tunnel(w, r net.Conn, b []byte, errs chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	defer closeWrite(w) //nolint:errcheck
	_, err := io.CopyBuffer(w, r, b)
	if err != nil {
		errs <- err
	}
}

func isTemporary(err error) bool {
	e, _ := err.(interface{ Temporary() bool })
	return e != nil && e.Temporary()
}
