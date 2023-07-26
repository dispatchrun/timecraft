package sandbox

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func (ns *LocalNamespace) socket(family Family, socktype Socktype, protocol Protocol) (*localSocket, error) {
	if protocol == 0 {
		switch socktype {
		case STREAM:
			protocol = TCP
		case DGRAM:
			protocol = UDP
		default:
			return nil, EPROTONOSUPPORT
		}
	}

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
	tunneled
	listening
	nonblocking
)

func (state localSocketState) is(s localSocketState) bool {
	return (state & s) != 0
}

func (state *localSocketState) set(s localSocketState) {
	*state |= s
}

func (state *localSocketState) unset(s localSocketState) {
	*state &= ^s
}

const (
	// Size of the buffer for addresses written on sockets: 20 is the minimum
	// size needed to store an IPv6 address, 2 bytes port number, and 2 bytes
	// address family. IPv4 addresses only use 8 bytes of the buffer but we
	// still serialize 20 bytes because working with fixed-size buffers gratly
	// simplifies the implementation.
	addrBufSize = 20
)

type localSocket struct {
	// Immutable properties of the socket; those are configured when the
	// socket is created, whether directly or when accepting on a server.
	ns       *LocalNamespace
	family   Family
	socktype Socktype
	protocol Protocol

	// State of the socket: its pair of file descriptors (fd0=read, fd1=write)
	// and a bit set tracking how the application configured it (whether it is
	// connected, listening, etc...).
	fd0   socketFD
	fd1   socketFD
	state localSocketState

	// The socket name and peer address; the name is set when the socket is
	// bound to a network interface, the peer is set when connecting the socket.
	//
	// We use atomic values because namespace of the same network may access the
	// socket concurrently and read those fields.
	name atomic.Value
	peer atomic.Value

	// Blocking sockets are implemented by lazily creating an *os.File on the
	// first time a socket enters a blocking operation to integrate with the Go
	// net poller using syscall.RawConn values constructed from those files.
	mutex    sync.Mutex
	file0    *os.File
	file1    *os.File
	rtimeout time.Duration
	wtimeout time.Duration

	// Buffers used for socket operations, retaining them reduces the number of
	// heap allocation on busy code paths.
	iovs    [][]byte
	addrBuf [addrBufSize]byte

	// The feilds below are used to manage bridges to external networks when the
	// parent namespace was configured with a dial or listen function.
	//
	// The error channel receives errors from the background goroutines passing
	// data back and forth between the socket and the external connections.
	conn   net.PacketConn
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

func (s *localSocket) Fd() uintptr {
	return uintptr(s.fd0.load())
}

func (s *localSocket) File() *os.File {
	f, _ := s.socketFile0()
	return f
}

func (s *localSocket) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.state.is(bound) {
		switch addr := s.name.Load().(type) {
		case *SockaddrInet4:
			s.ns.unlinkInet4(s, addr)
		case *SockaddrInet6:
			s.ns.unlinkInet6(s, addr)
		}
	}

	// When the socket was used in blocking mode, it lazily created an os.File
	// from a duplicate of its file descriptor in order to integrate with the
	// Go net poller.
	if s.file0 != nil {
		s.file0.Close()
	}
	if s.file1 != nil {
		s.file1.Close()
	}

	// First close the socket pair; if there are background goroutines managing
	// connections to external networks, this will interrupt the tunnels in
	// charge of passing data back and forth between the local socket fds and
	// the connections. Note that fd1 may have been detached if it was sent to
	// another socket to establish a connection.
	s.fd0.close()
	s.fd1.close()

	// When a packet listen function is configured on the parent namespace, the
	// socket may have created a bridge to accept inbound datagrams from other
	// networks so we have to close the net.PacketConn in order to interrupt the
	// goroutine in charge of receiving packets.
	if s.conn != nil {
		s.conn.Close()
	}

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
	sendSocketFd := s.fd1.acquire()
	if sendSocketFd < 0 {
		return EBADF
	}
	defer s.fd1.release(sendSocketFd)

	rbufsize, err := getsockoptInt(sendSocketFd, unix.SOL_SOCKET, unix.SO_RCVBUF)
	if err != nil {
		return err
	}
	wbufsize, err := getsockoptInt(sendSocketFd, unix.SOL_SOCKET, unix.SO_RCVBUF)
	if err != nil {
		return err
	}

	network := s.network()
	address := s.listenAddress()

	conn, err := s.ns.listenPacket(context.TODO(), network, address)
	if err != nil {
		return err
	}

	switch c := conn.(type) {
	case *net.UDPConn:
		_ = c.SetReadBuffer(rbufsize)
		_ = c.SetWriteBuffer(wbufsize)
	}

	errs := make(chan error, 1)
	s.errs = errs
	s.conn = conn

	buffer := make([]byte, rbufsize)
	// Increase reference count for fd1 because the goroutine will now share
	// ownership of the file descriptor.
	s.fd1.acquire()
	go func() {
		defer close(errs)
		defer conn.Close()
		defer s.fd1.release(sendSocketFd)

		var iovs [2][]byte
		var addrBuf [addrBufSize]byte
		// TODO: use optimizations like net.(*UDPConn).ReadMsgUDPAddrPort to
		// remove the heap allocation of the net.Addr returned by ReadFrom.
		for {
			n, addr, err := conn.ReadFrom(buffer)
			if err != nil {
				if err != io.EOF {
					errs <- err
				}
				return
			}

			addrBuf = encodeSockaddrAny(addr)
			iovs[0] = addrBuf[:]
			iovs[1] = buffer[:n]

			if _, err := sendto(sendSocketFd, iovs[:], nil, 0); err != nil {
				errs <- err
				return
			}
		}
	}()
	return nil
}

func (s *localSocket) listen() error {
	network := s.network()
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

	if s.socktype != DGRAM {
		serverFd := server.fd1.acquire()
		if serverFd < 0 {
			return ECONNREFUSED
		}
		defer server.fd1.release(serverFd)

		if err := s.connect(serverFd, s.name.Load()); err != nil {
			return err
		}
	}

	s.peer.Store(addr)
	s.state.set(connected)

	if s.socktype != DGRAM && s.state.is(nonblocking) {
		return EINPROGRESS
	}
	return nil
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
	if err := sendmsg(serverFd, addrBuf[:], rights, nil, 0); err != nil {
		return ECONNREFUSED
	}
	s.fd1.close()
	return nil
}

func (s *localSocket) dial(addr Sockaddr) error {
	// When using datagram sockets with a packet listen function setup, the
	// connection is emulated by simply setting the peer address since a packet
	// tunnel has already been constructed, all we care about is making sure the
	// socket only exchange datagrams with the address it is connected to.
	if s.conn != nil {
		s.peer.Store(addr)
		s.state.set(connected)
		return nil
	}

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

	network := s.network()
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
	s.state.set(connected | tunneled)

	if s.state.is(nonblocking) {
		return EINPROGRESS
	}
	return nil
}

func (s *localSocket) network() string {
	switch s.socktype {
	case STREAM:
		switch s.family {
		case INET:
			return "tcp4"
		case INET6:
			return "tcp6"
		default:
			return "unix"
		}
	default: // DGRAM
		switch s.family {
		case INET:
			return "udp4"
		case INET6:
			return "udp6"
		default:
			return "unixgram"
		}
	}
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
	if err := s.Error(); err != nil {
		return nil, nil, err
	}

	socket := &localSocket{
		ns:       s.ns,
		family:   s.family,
		socktype: s.socktype,
		protocol: s.protocol,
		state:    bound | accepted | connected,
	}

	var err error
	var oobn int
	var oobBuf [24]byte
	var addrBuf [addrBufSize]byte

	cmsg := oobBuf[:unix.CmsgSpace(1)]
	if s.state.is(nonblocking) {
		_, oobn, _, _, err = recvmsg(fd, addrBuf[:], cmsg, 0)
	} else {
		rawConn, err := s.syscallConn0()
		if err != nil {
			return nil, nil, err
		}
		rawConnErr := rawConn.Read(func(fd uintptr) bool {
			_, oobn, _, _, err = recvmsg(int(fd), addrBuf[:], cmsg, 0)
			if err != nil {
				if !s.state.is(nonblocking) {
					err = nil
					return false
				}
			}
			return true
		})
		if err == nil {
			err = rawConnErr
		}
	}
	if err != nil {
		return nil, nil, handleSocketIOError(err)
	}

	// TOOD: remove the heap allocation for the return value by implementing
	// ParseSocketControlMessage; we know that we will receive at most most one
	// message since we sized the buffer accordingly.
	msgs, err := unix.ParseSocketControlMessage(oobBuf[:oobn])
	if err != nil {
		return nil, nil, err
	}
	if len(msgs) == 0 {
		return nil, nil, ECONNABORTED
	}
	if len(msgs) > 1 {
		println("BUG: accept received fmore than one file descriptor")
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
	if err := s.Error(); err != nil {
		return -1, 0, nil, err
	}
	if !s.state.is(bound) {
		if err := s.bindAny(); err != nil {
			return -1, 0, nil, err
		}
	}

	if s.socktype == DGRAM && !s.state.is(tunneled) {
		s.iovs = s.iovs[:0]
		s.iovs = append(s.iovs, s.addrBuf[:])
		s.iovs = append(s.iovs, iovs...)
		iovs = s.iovs
		defer clearIOVecs(iovs)
	}

	if s.state.is(nonblocking) {
		for {
			n, rflags, addr, err := recvfrom(int(fd), iovs, flags)
			return s.handleRecvFrom(n, rflags, addr, err)
		}
	}

	rawConn, err := s.syscallConn0()
	if err != nil {
		return -1, 0, nil, err
	}

	var n, rflags int
	var addr Sockaddr
	for {
		rawConnErr := rawConn.Read(func(fd uintptr) bool {
			n, rflags, addr, err = recvfrom(int(fd), iovs, flags)
			if err != EAGAIN {
				return true
			}
			err = nil
			return false
		})
		if err == nil {
			err = rawConnErr
		}
		n, rflags, addr, err = s.handleRecvFrom(n, rflags, addr, err)
		if n >= 0 || err != nil {
			return n, rflags, addr, err
		}
	}
}

func (s *localSocket) handleRecvFrom(n, rflags int, addr Sockaddr, err error) (int, int, Sockaddr, error) {
	if err == nil && s.socktype == DGRAM && !s.state.is(tunneled) {
		addr = decodeSockaddr(s.addrBuf)
		n -= addrBufSize
		// Connected datagram sockets may receive data from addresses that
		// they are not connected to, those datagrams should be dropped.
		if s.state.is(connected) {
			recvAddrPort := SockaddrAddrPort(addr)
			peerAddrPort := SockaddrAddrPort(s.peer.Load().(Sockaddr))
			if recvAddrPort != peerAddrPort {
				return -1, 0, nil, nil
			}
		}
	}
	return n, rflags, addr, handleSocketIOError(err)
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
	if err := s.Error(); err != nil {
		return -1, err
	}
	if !s.state.is(bound) {
		if err := s.bindAny(); err != nil {
			return -1, err
		}
	}

	if s.socktype == DGRAM && !s.state.is(tunneled) && addr == nil {
		switch peer := s.peer.Load().(type) {
		case *SockaddrInet4:
			addr = peer
		case *SockaddrInet6:
			addr = peer
		}
	}

	sendSocket, sendSocketFd := s, fd
	// We only perform a lookup of the peer socket if an address is provided,
	// which means that the socket is not connected to a particular destination
	// (it must be a datagram socket). This may result in sending the datagram
	// directly to the peer socket's file descriptor, or passing it to a packet
	// connection if one was opened by the socket.
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
			// When the destination is not an address within the local network,
			// but a packet listen function was setup then the socket has opened
			// a packet connection that we use to send the datagram to a remote
			// address. Note that we do expect that writing datagrams is not a
			// blocking operation and the net.PacketConn may drop packets.
			if err == ENETUNREACH && s.conn != nil {
				return writeTo(s.conn, iovs, addr)
			}
			return -1, err
		}

		// If the application tried to send a datagram to a socket which is not
		// a datagram socket, we drop the data here pretending that we were able
		// to send it.
		if peer.socktype != DGRAM {
			return iovecLen(iovs), nil
		}

		// There are two reasons why the peer's second file descriptor may not
		// be available here: the peer could have been closed concurrently, or
		// it may have been connected to a specific address which indicates
		// that it is not a listening socket.
		peerFd := peer.fd1.acquire()
		if peerFd < 0 {
			return -1, EHOSTUNREACH
		}
		defer peer.fd1.release(peerFd)
		sendSocket, sendSocketFd = peer, peerFd
	}

	if s.socktype == DGRAM && !s.state.is(tunneled) {
		s.addrBuf = encodeSockaddrAny(s.name.Load())
		s.iovs = s.iovs[:0]
		s.iovs = append(s.iovs, s.addrBuf[:])
		s.iovs = append(s.iovs, iovs...)
		iovs = s.iovs
		defer clearIOVecs(iovs)
	}

	var n int
	var err error

	if s.state.is(nonblocking) {
		n, err = sendto(sendSocketFd, iovs, nil, flags)
	} else {
		var rawConn syscall.RawConn
		// When sending to the socket, we write to fd0 because we want the other
		// side to receive the data (for connected sockets).
		//
		// The other condition happens when writing to datagram sockets, in that
		// case we write directly to the other end of the destination socket.
		if sendSocket == s {
			rawConn, err = sendSocket.syscallConn0()
		} else {
			rawConn, err = sendSocket.syscallConn1()
		}
		if err != nil {
			return -1, err
		}
		rawConnErr := rawConn.Write(func(fd uintptr) bool {
			n, err = sendto(int(fd), iovs, nil, flags)
			if err != EAGAIN {
				return true
			}
			err = nil
			return false
		})
		if err == nil {
			err = rawConnErr
		}
	}

	if n > 0 && addr != nil {
		n -= addrBufSize
	}
	return n, handleSocketIOError(err)
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

func (s *localSocket) Error() error {
	select {
	case err := <-s.errs:
		return err
	default:
		return nil
	}
}

func (s *localSocket) IsListening() (bool, error) {
	return s.state.is(listening), nil
}

func (s *localSocket) IsNonBlock() (bool, error) {
	return s.state.is(nonblocking), nil
}

func (s *localSocket) RecvBuffer() (int, error) {
	return s.getOptInt(unix.SOL_SOCKET, unix.SO_RCVBUF)
}

func (s *localSocket) SendBuffer() (int, error) {
	return s.getOptInt(unix.SOL_SOCKET, unix.SO_SNDBUF)
}

func (s *localSocket) TCPNoDelay() (bool, error) {
	return false, EOPNOTSUPP
}

func (s *localSocket) RecvTimeout() (time.Duration, error) {
	return s.rtimeout, nil
}

func (s *localSocket) SendTimeout() (time.Duration, error) {
	return s.wtimeout, nil
}

func (s *localSocket) SetNonBlock(nonblock bool) error {
	if nonblock {
		s.state.set(nonblocking)
	} else {
		s.state.unset(nonblocking)
	}
	return nil
}

func (s *localSocket) SetRecvBuffer(size int) error {
	return s.setOptInt(unix.SOL_SOCKET, unix.SO_RCVBUF, size)
}

func (s *localSocket) SetSendBuffer(size int) error {
	return s.setOptInt(unix.SOL_SOCKET, unix.SO_SNDBUF, size)
}

func (s *localSocket) SetRecvTimeout(timeout time.Duration) error {
	s.rtimeout = timeout
	return nil
}

func (s *localSocket) SetSendTimeout(timeout time.Duration) error {
	s.wtimeout = timeout
	return nil
}

func (s *localSocket) SetTCPNoDelay(nodelay bool) error {
	return EOPNOTSUPP
}

func (s *localSocket) SetTLSServerName(serverName string) (err error) {
	if s.htls == nil {
		return EINVAL
	}
	s.htls <- serverName
	s.htlsClear()
	return nil
}

func (s *localSocket) getOptInt(level, name int) (int, error) {
	fd := s.fd0.acquire()
	if fd < 0 {
		return -1, EBADF
	}
	defer s.fd0.release(fd)
	return getsockoptInt(fd, level, name)
}

func (s *localSocket) setOptInt(level, name, value int) error {
	fd := s.fd0.acquire()
	if fd < 0 {
		return EBADF
	}
	defer s.fd0.release(fd)
	return setsockoptInt(fd, level, name, value)
}

func (s *localSocket) htlsClear() {
	if s.htls != nil {
		close(s.htls)
		s.htls = nil
	}
}

func (s *localSocket) syscallConn0() (syscall.RawConn, error) {
	f, err := s.socketFile0()
	if err != nil {
		return nil, err
	}
	if err := setFileDeadline(f, s.rtimeout, s.wtimeout); err != nil {
		return nil, err
	}
	return f.SyscallConn()
}

func (s *localSocket) syscallConn1() (syscall.RawConn, error) {
	f, err := s.socketFile1()
	if err != nil {
		return nil, err
	}
	if err := setFileDeadline(f, s.rtimeout, s.wtimeout); err != nil {
		return nil, err
	}
	return f.SyscallConn()
}

func (s *localSocket) socketFile0() (*os.File, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.file0 != nil {
		return s.file0, nil
	}
	fd := s.fd0.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.fd0.release(fd)
	fileFd, err := dup(fd)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fileFd), "")
	s.file0 = f
	return f, nil
}

func (s *localSocket) socketFile1() (*os.File, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.file1 != nil {
		return s.file1, nil
	}
	fd := s.fd1.acquire()
	if fd < 0 {
		return nil, EBADF
	}
	defer s.fd1.release(fd)
	fileFd, err := dup(fd)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fileFd), "")
	s.file1 = f
	return f, nil
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

func iovecBuf(iovs [][]byte) []byte {
	buf := make([]byte, 0, iovecLen(iovs))
	for _, iov := range iovs {
		buf = append(buf, iov...)
	}
	return buf
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

// writeTo writes a datagram represented by the given I/O vector to a destination
// address on a packet connection.
//
// If the net.PacketConn is an instance of *net.UDPConn, the function uses
// optimized methods to avoid heap allocations for the intermediary net.Addr
// value that must be constructed.
func writeTo(conn net.PacketConn, iovs [][]byte, addr Sockaddr) (int, error) {
	buf := iovecBuf(iovs) // TODO: pool this buffer?
	addrPort := SockaddrAddrPort(addr)
	switch c := conn.(type) {
	case *net.UDPConn:
		return c.WriteToUDPAddrPort(buf, addrPort)
	default:
		addr := addrPort.Addr()
		port := addrPort.Port()
		return c.WriteTo(buf, &net.UDPAddr{IP: addr.AsSlice(), Port: int(port)})
	}
}
