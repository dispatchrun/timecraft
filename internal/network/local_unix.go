package network

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"sync/atomic"

	"github.com/stealthrocket/timecraft/internal/htls"
	"golang.org/x/sys/unix"
)

func (ns *LocalNamespace) socket(family Family, socktype Socktype, protocol Protocol) (*localSocket, error) {
	socket := &localSocket{
		ns:       ns,
		family:   family,
		socktype: socktype,
		protocol: protocol,
	}
	fds, err := socketpair(int(UNIX), int(socktype), 0)
	if err != nil {
		return nil, err
	}
	socket.fd0.init(fds[0])
	socket.fd1.init(fds[1])
	return socket, nil
}

type localSocketState uint8

const (
	bound localSocketState = 1 << iota
	accepted
	connected
	listening
)

func (state localSocketState) is(s localSocketState) bool {
	return (state & s) != 0
}

func (state *localSocketState) set(s localSocketState) {
	*state |= s
}

const (
	addrBufSize = 20
)

type localSocket struct {
	ns       *LocalNamespace
	family   Family
	socktype Socktype
	protocol Protocol

	fd0   socketFD
	fd1   socketFD
	state localSocketState

	name atomic.Value
	peer atomic.Value

	iovs    [][]byte
	addrBuf [addrBufSize]byte

	lstn   net.Listener
	htls   chan<- string
	errs   <-chan error
	cancel context.CancelFunc
}

func (s *localSocket) String() string {
	name := s.name.Load()
	peer := s.peer.Load()
	if name == nil {
		return "?"
	}
	if peer == nil {
		return SockaddrAddrPort(name.(Sockaddr)).String()
	}
	nameString := SockaddrAddrPort(name.(Sockaddr)).String()
	peerString := SockaddrAddrPort(peer.(Sockaddr)).String()
	if s.state.is(accepted) {
		return peerString + "->" + nameString
	} else {
		return nameString + "->" + peerString
	}
}

func (s *localSocket) Family() Family {
	return s.family
}

func (s *localSocket) Type() Socktype {
	return s.socktype
}

func (s *localSocket) Fd() int {
	return s.fd0.load()
}

func (s *localSocket) Close() error {
	if s.state.is(bound) {
		switch addr := s.name.Load().(type) {
		case *SockaddrInet4:
			s.ns.unlinkInet4(s, addr)
		case *SockaddrInet6:
			s.ns.unlinkInet6(s, addr)
		}
	}

	// First close the socket pair; if there are background goroutines managing
	// connections to external networks, this will interrupt the tunnels in
	// charge of passing data back and forth between the local socket fds and
	// the connections. Note that fd1 may have been detached if it was sent to
	// another socket to establish a connection.
	s.fd0.close()
	s.fd1.close()

	// When a listen function is configured on the parent namespace, the socket
	// may have created a bridge to accept inbound connections from other
	// networks so we have to close the net.Listener in order to interrupt the
	// goroutine in charge of accepting connections.
	if s.lstn != nil {
		s.lstn.Close()
	}

	// When a dial function isconfigured on the parent namespace, the socket may
	// be in the process of establishing an outbound connection; in that case,
	// a context was created to control asynchronous cancellation of the dial
	// and we must invoke the cancellation function to interrupt it.
	if s.cancel != nil {
		s.cancel()
	}

	// When either a listen or dial functions were set on the parent namespace
	// and the socket has created bridges to external networks, an error channel
	// was set to receive errors from the background goroutines managing those
	// connections. Because we arleady interrupted asynchrononus operations, we
	// have the guarantee that the channel will be closed when the goroutines
	// exit, so the for loop will eventually stop. Flushing the error channel is
	// necessary to ensure that none of the background goroutines remain blocked
	// attempting to produce to the errors channel.
	if s.errs != nil {
		for range s.errs {
		}
	}
	return nil
}

func (s *localSocket) Bind(addr Sockaddr) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)

	if s.state.is(bound) {
		return EINVAL
	}

	switch bind := addr.(type) {
	case *SockaddrInet4:
		if s.family == INET {
			return s.bindInet4(bind)
		}
	case *SockaddrInet6:
		if s.family == INET6 {
			return s.bindInet6(bind)
		}
	}
	return EAFNOSUPPORT
}

func (s *localSocket) bindAny() error {
	switch s.family {
	case INET:
		return s.bindInet4(&sockaddrInet4Any)
	default:
		return s.bindInet6(&sockaddrInet6Any)
	}
}

func (s *localSocket) bindInet4(addr *SockaddrInet4) error {
	if err := s.ns.bindInet4(s, addr); err != nil {
		return err
	}
	if s.socktype == DGRAM && s.ns.listenPacket != nil {
		return s.listenPacket()
	}
	return nil
}

func (s *localSocket) bindInet6(addr *SockaddrInet6) error {
	if err := s.ns.bindInet6(s, addr); err != nil {
		return err
	}
	if s.socktype == DGRAM && s.ns.listenPacket != nil {
		return s.listenPacket()
	}
	return nil
}

func (s *localSocket) Listen(backlog int) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)

	if s.state.is(listening) {
		return nil
	}
	if s.state.is(connected) {
		return EINVAL
	}
	if s.socktype != STREAM {
		return EINVAL
	}
	if !s.state.is(bound) {
		if err := s.bindAny(); err != nil {
			return err
		}
	}
	if s.ns.listen != nil {
		if err := s.listen(); err != nil {
			return err
		}
	}
	s.state.set(listening)
	return nil
}

func (s *localSocket) listenAddress() string {
	switch a := s.name.Load().(type) {
	case *SockaddrInet4:
		return fmt.Sprintf(":%d", a.Port)
	case *SockaddrInet6:
		return fmt.Sprintf("[::]:%d", a.Port)
	default:
		return ""
	}
}

func (s *localSocket) listenPacket() error {
	// TODO: figure out how to tunnel packet connections in a way that allows
	// for both internal and external packets to transit.

	// fd := s.fd1.acquire()
	// if fd < 0 {
	// 	return EBADF
	// }
	// defer s.fd1.release(fd)

	// network := s.protocol.Network()
	// address := s.listenAddress()

	// upstream, err := s.ns.listenPacket(context.TODO(), network, address)
	// if err != nil {
	// 	return err
	// }

	// f := os.NewFile(uintptr(fd), "")
	// defer f.Close()
	// s.fd1.acquire()
	// s.fd1.close()

	// downstream, err := net.FilePacketConn(f)
	// if err != nil {
	// 	upstream.Close()
	// 	return err
	// }

	// go func() {
	// 	defer downstream.Close()
	// 	defer upstream.Close()

	// }()
	return nil
}

func (s *localSocket) listen() error {
	network := s.protocol.Network()
	address := s.listenAddress()

	l, err := s.ns.listen(context.TODO(), network, address)
	if err != nil {
		return err
	}

	errs := make(chan error, 1)
	s.lstn = l
	s.errs = errs

	go func() {
		defer close(errs)
		for {
			c, err := l.Accept()
			if err != nil {
				e, _ := err.(interface{ Temporary() bool })
				if e == nil || !e.Temporary() {
					errs <- err
					return
				}
			} else if s.serve(c) != nil {
				c.Close()
				select {
				case errs <- ECONNABORTED:
				default:
				}
			}
		}
	}()
	return nil
}

func (s *localSocket) serve(upstream net.Conn) error {
	serverFd := s.fd1.acquire()
	if serverFd < 0 {
		return EBADF
	}
	defer s.fd1.release(serverFd)

	rbufsize, err := getsockoptInt(serverFd, unix.SOL_SOCKET, unix.SO_RCVBUF)
	if err != nil {
		return err
	}
	wbufsize, err := getsockoptInt(serverFd, unix.SOL_SOCKET, unix.SO_SNDBUF)
	if err != nil {
		return err
	}
	socket, err := s.ns.socket(s.family, s.socktype, s.protocol)
	if err != nil {
		return err
	}
	defer socket.Close()

	if err := socket.connect(serverFd, upstream.RemoteAddr()); err != nil {
		return err
	}

	f := os.NewFile(uintptr(socket.fd0.acquire()), "")
	defer f.Close()
	socket.fd0.close() // detach from the socket, f owns the fd now

	downstream, err := net.FileConn(f)
	if err != nil {
		return err
	}

	go func() {
		defer upstream.Close()
		defer downstream.Close()

		_ = tunnel(downstream, upstream, rbufsize, wbufsize)
		// TODO: figure out if this error needs to be reported:
		//
		// When the downstream file is closed, the other end of the
		// connection will observe that the socket was shutdown, we
		// lose the information of why but we currently have no way
		// of locating the peer socket on the other side.
	}()
	return nil
}

func (s *localSocket) Connect(addr Sockaddr) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)

	if s.state.is(listening) {
		return EINVAL
	}
	if s.state.is(connected) {
		return EISCONN
	}
	if !s.state.is(bound) {
		if err := s.bindAny(); err != nil {
			return err
		}
	}

	var server *localSocket
	var err error
	switch a := addr.(type) {
	case *SockaddrInet4:
		server, err = s.ns.lookupInet4(s, a)
	case *SockaddrInet6:
		server, err = s.ns.lookupInet6(s, a)
	default:
		return EAFNOSUPPORT
	}
	if err != nil {
		if err == ENETUNREACH {
			return s.dial(addr)
		}
		return ECONNREFUSED
	}
	if server.socktype != s.socktype {
		return ECONNREFUSED
	}

	if s.socktype == DGRAM {
		s.peer.Store(addr)
		s.state.set(connected)
		return nil
	}

	serverFd := server.fd1.acquire()
	if serverFd < 0 {
		return ECONNREFUSED
	}
	defer server.fd1.release(serverFd)

	if err := s.connect(serverFd, s.name.Load()); err != nil {
		return err
	}

	s.peer.Store(addr)
	s.state.set(connected)
	return EINPROGRESS
}

func (s *localSocket) connect(serverFd int, addr any) error {
	fd1 := s.fd1.acquire()
	if fd1 < 0 {
		return EBADF
	}
	defer s.fd1.release(fd1)

	addrBuf := encodeSockaddrAny(addr)
	// TODO: remove the heap allocation by implementing UnixRights to output to
	// a stack buffer.
	rights := unix.UnixRights(fd1)
	if err := unix.Sendmsg(serverFd, addrBuf[:], rights, nil, 0); err != nil {
		return ECONNREFUSED
	}
	s.fd1.close()
	return nil
}

func (s *localSocket) dial(addr Sockaddr) error {
	dial := s.ns.dial
	if dial == nil {
		return ENETUNREACH
	}

	fd1 := s.fd1.acquire()
	if fd1 < 0 {
		return EBADF
	}
	defer s.fd1.release(fd1)

	// TODO:
	// - remove the 2x factor that linux applies on socket buffers
	// - do the two ends of a socket pair share the same buffer sizes?
	rbufsize, err := getsockoptInt(fd1, unix.SOL_SOCKET, unix.SO_RCVBUF)
	if err != nil {
		return err
	}
	wbufsize, err := getsockoptInt(fd1, unix.SOL_SOCKET, unix.SO_SNDBUF)
	if err != nil {
		return err
	}

	f := os.NewFile(uintptr(fd1), "")
	defer f.Close()
	s.fd1.acquire()
	s.fd1.close() // detach from the socket, f owns the fd now

	downstream, err := net.FileConn(f)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel

	htls := make(chan string, 1)
	s.htls = htls

	errs := make(chan error, 1)
	s.errs = errs

	network := s.protocol.Network()
	address := SockaddrAddrPort(addr).String()
	go func() {
		defer close(errs)
		defer downstream.Close()

		upstream, err := dial(ctx, network, address)
		if err != nil {
			errs <- err
			return
		}
		defer upstream.Close()

		select {
		case <-ctx.Done():
			errs <- ctx.Err()
			return
		case serverName, ok := <-htls:
			if !ok {
				break
			}
			tlsConn := tls.Client(upstream, &tls.Config{
				ServerName: serverName,
			})
			if err := tlsConn.HandshakeContext(ctx); err != nil {
				errs <- err
				return
			}
			upstream = tlsConn
		}

		if err := tunnel(downstream, upstream, rbufsize, wbufsize); err != nil {
			errs <- err
		}
	}()

	s.peer.Store(addr)
	s.state.set(connected)
	return EINPROGRESS
}

func (s *localSocket) Accept() (Socket, Sockaddr, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return nil, nil, EBADF
	}
	defer s.fd0.release(fd)

	if !s.state.is(listening) {
		return nil, nil, EINVAL
	}
	if err := s.getError(); err != nil {
		return nil, nil, err
	}

	socket := &localSocket{
		ns:       s.ns,
		family:   s.family,
		socktype: s.socktype,
		protocol: s.protocol,
		state:    bound | accepted | connected,
	}

	var oobn int
	var oobBuf [24]byte
	var addrBuf [addrBufSize]byte
	for {
		var err error
		// TOOD: remove the heap allocation for the receive address by
		// implementing recvmsg and using the stack-allocated socket address
		// buffer.
		_, oobn, _, _, err = unix.Recvmsg(fd, addrBuf[:], oobBuf[:unix.CmsgSpace(1)], 0)
		if err == nil {
			break
		}
		if err != EINTR {
			return nil, nil, err
		}
		if oobn > 0 {
			break
		}
	}

	// TOOD: remove the heap allocation for the return value by implementing
	// ParseSocketControlMessage; we know that we will receive at most most one
	// message since we sized the buffer accordingly.
	msgs, err := unix.ParseSocketControlMessage(oobBuf[:oobn])
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove the heap allocation for the return fd slice by implementing
	// ParseUnixRights and decoding the single file descriptor we received in a
	// stack-allocated variabled.
	fds, err := unix.ParseUnixRights(&msgs[0])
	if err != nil {
		return nil, nil, err
	}

	addr := decodeSockaddr(addrBuf)
	socket.fd0.init(fds[0])
	socket.fd1.init(-1)
	socket.name.Store(s.name.Load())
	socket.peer.Store(addr)
	return socket, addr, nil
}

func (s *localSocket) Name() (Sockaddr, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.fd0.release(fd)
	switch name := s.name.Load().(type) {
	case *SockaddrInet4:
		return name, nil
	case *SockaddrInet6:
		return name, nil
	}
	switch s.family {
	case INET:
		return &sockaddrInet4Any, nil
	default:
		return &sockaddrInet6Any, nil
	}
}

func (s *localSocket) Peer() (Sockaddr, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.fd0.release(fd)
	switch peer := s.peer.Load().(type) {
	case *SockaddrInet4:
		return peer, nil
	case *SockaddrInet6:
		return peer, nil
	}
	return nil, ENOTCONN
}

func (s *localSocket) RecvFrom(iovs [][]byte, flags int) (int, int, Sockaddr, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return -1, 0, nil, EBADF
	}
	defer s.fd0.release(fd)
	s.htlsClear()

	if s.state.is(listening) {
		return -1, 0, nil, EINVAL
	}
	if err := s.getError(); err != nil {
		return -1, 0, nil, err
	}
	if !s.state.is(bound) {
		if err := s.bindAny(); err != nil {
			return -1, 0, nil, err
		}
	}

	if s.socktype == DGRAM {
		s.iovs = s.iovs[:0]
		s.iovs = append(s.iovs, s.addrBuf[:])
		s.iovs = append(s.iovs, iovs...)
		iovs = s.iovs
		defer clearIOVecs(iovs)
	}

	// TODO: remove the heap allocation that happens for the socket address by
	// implementing recvfrom(2) and using a cached socket address for connected
	// sockets.
	for {
		n, _, rflags, _, err := unix.RecvmsgBuffers(fd, iovs, nil, flags)
		if err == EINTR {
			if n == 0 {
				continue
			}
			err = nil
		}
		var addr Sockaddr
		if s.socktype == DGRAM {
			addr = decodeSockaddr(s.addrBuf)
			n -= addrBufSize
		}
		return n, rflags, addr, err
	}
}

func (s *localSocket) SendTo(iovs [][]byte, addr Sockaddr, flags int) (int, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.fd0.release(fd)
	s.htlsClear()

	if s.state.is(listening) {
		return -1, EINVAL
	}
	if s.state.is(connected) && addr != nil {
		return -1, EISCONN
	}
	if !s.state.is(connected) && addr == nil {
		return -1, ENOTCONN
	}
	if err := s.getError(); err != nil {
		return -1, err
	}
	if !s.state.is(bound) {
		if err := s.bindAny(); err != nil {
			return -1, err
		}
	}

	sendSocketFd := fd
	if addr != nil {
		var peer *localSocket
		var err error
		switch a := addr.(type) {
		case *SockaddrInet4:
			peer, err = s.ns.lookupInet4(s, a)
		case *SockaddrInet6:
			peer, err = s.ns.lookupInet6(s, a)
		default:
			return -1, EAFNOSUPPORT
		}
		if err != nil {
			return -1, err
		}
		if peer.socktype != DGRAM {
			return iovecLen(iovs), nil
		}

		peerFd := peer.fd1.acquire()
		if peerFd < 0 {
			return -1, EHOSTUNREACH
		}
		defer peer.fd1.release(peerFd)
		sendSocketFd = peerFd

		s.addrBuf = encodeSockaddrAny(s.name.Load())
		s.iovs = s.iovs[:0]
		s.iovs = append(s.iovs, s.addrBuf[:])
		s.iovs = append(s.iovs, iovs...)
		iovs = s.iovs
		defer clearIOVecs(iovs)
	}

	for {
		n, err := unix.SendmsgBuffers(sendSocketFd, iovs, nil, nil, flags)
		if err == EINTR {
			if n == 0 {
				continue
			}
			err = nil
		}
		if n > 0 && s.socktype == DGRAM {
			n -= addrBufSize
		}
		return n, err
	}
}

func (s *localSocket) Shutdown(how int) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)
	defer s.htlsClear()
	return shutdown(fd, how)
}

func (s *localSocket) GetOptInt(level, name int) (int, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.fd0.release(fd)
	return getsockoptInt(fd, level, name)
}

func (s *localSocket) GetOptString(level, name int) (string, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return "", EBADF
	}
	defer s.fd0.release(fd)

	switch level {
	case htls.Level:
		switch name {
		case htls.ServerName:
			return "", EINVAL
		default:
			return "", ENOPROTOOPT
		}
	}

	return getsockoptString(fd, level, name)
}

func (s *localSocket) SetOptInt(level, name, value int) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)
	return setsockoptInt(fd, level, name, value)
}

func (s *localSocket) SetOptString(level, name int, value string) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)

	switch level {
	case htls.Level:
		switch name {
		case htls.ServerName:
			return s.htlsSetServerName(value)
		default:
			return ENOPROTOOPT
		}
	}

	return setsockoptString(fd, level, name, value)
}

func (s *localSocket) getError() error {
	select {
	case err := <-s.errs:
		return err
	default:
		return nil
	}
}

func (s *localSocket) htlsClear() {
	if s.htls != nil {
		close(s.htls)
	}
}

func (s *localSocket) htlsSetServerName(hostname string) (err error) {
	defer func() {
		if recover() != nil {
			if s.htls == nil {
				err = EINVAL
			} else {
				err = EISCONN
			}
		}
	}()
	s.htls <- hostname
	close(s.htls)
	return nil
}

func clearIOVecs(iovs [][]byte) {
	for i := range iovs {
		iovs[i] = nil
	}
}

func encodeSockaddrAny(addr any) (buf [addrBufSize]byte) {
	switch a := addr.(type) {
	case *SockaddrInet4:
		return encodeAddrPortInet4(a.Addr, a.Port)
	case *SockaddrInet6:
		return encodeAddrPortInet6(a.Addr, a.Port)
	case *net.TCPAddr:
		return encodeAddrPortIP(a.IP, a.Port)
	case *net.UDPAddr:
		return encodeAddrPortIP(a.IP, a.Port)
	default:
		return
	}
}

func encodeAddrPortIP(addr net.IP, port int) (buf [addrBufSize]byte) {
	if ipv4 := addr.To4(); ipv4 != nil {
		return encodeAddrPortInet4(([4]byte)(ipv4), port)
	} else {
		return encodeAddrPortInet6(([16]byte)(addr), port)
	}
}

func encodeAddrPortInet4(addr [4]byte, port int) (buf [addrBufSize]byte) {
	binary.LittleEndian.PutUint16(buf[0:2], uint16(INET))
	binary.LittleEndian.PutUint16(buf[2:4], uint16(port))
	*(*[4]byte)(buf[4:]) = addr
	return
}

func encodeAddrPortInet6(addr [16]byte, port int) (buf [addrBufSize]byte) {
	binary.LittleEndian.PutUint16(buf[0:2], uint16(INET6))
	binary.LittleEndian.PutUint16(buf[2:4], uint16(port))
	*(*[16]byte)(buf[4:]) = addr
	return
}

func decodeSockaddr(buf [addrBufSize]byte) Sockaddr {
	switch Family(binary.LittleEndian.Uint16(buf[0:2])) {
	case INET:
		return decodeSockaddrInet4(buf)
	default:
		return decodeSockaddrInet6(buf)
	}
}

func decodeSockaddrInet4(buf [addrBufSize]byte) *SockaddrInet4 {
	return &SockaddrInet4{
		Port: int(binary.LittleEndian.Uint16(buf[2:4])),
		Addr: ([4]byte)(buf[4:]),
	}
}

func decodeSockaddrInet6(buf [addrBufSize]byte) *SockaddrInet6 {
	return &SockaddrInet6{
		Port: int(binary.LittleEndian.Uint16(buf[2:4])),
		Addr: ([16]byte)(buf[4:]),
	}
}

func iovecLen(iovs [][]byte) (n int) {
	for _, iov := range iovs {
		n += len(iov)
	}
	return n
}
