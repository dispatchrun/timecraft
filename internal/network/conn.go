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

func tunnel(downstream, upstream net.Conn, rbufsize, wbufsize int) error {
	buffer := make([]byte, rbufsize+wbufsize) // TODO: pool this buffer?
	errs := make(chan error, 2)
	wg := new(sync.WaitGroup)
	wg.Add(2)

	go copyAndClose(downstream, upstream, buffer[:rbufsize], errs, wg)
	go copyAndClose(upstream, downstream, buffer[rbufsize:], errs, wg)

	wg.Wait()
	close(errs)
	return <-errs
}

func copyAndClose(w, r net.Conn, b []byte, errs chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	defer closeWrite(w) //nolint:errcheck
	_, err := io.CopyBuffer(w, r, b)
	if err != nil {
		errs <- err
	}
}
