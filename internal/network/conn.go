package network

import (
	"io"
	"net"
	"sync"
)

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
	_, err := io.CopyBuffer(w, r, b)
	if err != nil {
		errs <- err
	}
	switch c := w.(type) {
	case interface{ CloseWrite() error }:
		c.CloseWrite() //nolint:errcheck
	default:
		c.Close()
	}
	wg.Done()
}

/*
func packetTunnel(downstream, upstream net.PacketConn, rbufsize, wbufsize int) error {
	buffer := make([]byte, 2*addrBufSize+rbufsize+wbufsize) // TODO: pool this buffer?
	errs := make(chan error, 2)
	wg := new(sync.WaitGroup)
	wg.Add(2)

	go packetCopyAndCloseInbound(downstream, upstream, buffer[:addrBufSize+rbufsize], errs, wg)
	go packetCopyAndCloseOutbound(upstream, downstream, buffer[addrBufSize+rbufsize:], errs, wg)

	wg.Wait()
	close(errs)
	return <-errs
}

func packetCopyAndCloseInbound(w, r net.PacketConn, b []byte, errs chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	defer closeWrite(w)

	for {
		n, addr, err := r.ReadFrom(b[addrBufSize:])
		if err != nil {
			if err != io.EOF {
				errs <- err
			}
			return
		}

		addrBuf := encodeSockaddrAny(addr)
		copy(b, addrBuf[:])

		_, err := w.WriteTo(b[:addrBufSize+n], nil)
		if err != nil {
			errs <- err
			return
		}
	}
}

func packetCopyAndCloseOutbound(w, r net.PacketConn, b []byte, errs chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	defer closeWrite(w)

	var addrBuf = b[:addrBufSize]
	var dstAddr net.UDPAddr
	for {
		n, _, err := r.ReadFrom(b)
		if err != nil {
			if err != io.EOF {
				errs <- err
			}
			return
		}

		dstAddr.Port = int(binary.LittleEndian.Uint16(addrBuf[2:4]))
		switch Family(binary.LittleEndian.Uint16(addrBuf[0:2])) {
		case INET:
			dstAddr.IP = net.IP(addrBuf[4:8])
		default:
			dstAddr.IP = net.IP(addrBuf[4:20])
		}

		_, err := w.WriteTo(b[addrBufSize:addrBufSize+n], &dstAddr)
		if err != nil {
			errs <- err
			return
		}
	}
}
*/
