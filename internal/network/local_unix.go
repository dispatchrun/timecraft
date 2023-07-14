package network

import (
	"encoding/binary"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

func (ns *localNamespace) Socket(family Family, socktype Socktype, protocol Protocol) (Socket, error) {
	switch family {
	case INET, INET6:
	default:
		return ns.host.Socket(family, socktype, protocol)
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
	connected
	listening
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
	addrBufSize = 20
)

type localSocket struct {
	ns       *localNamespace
	fd0      socketFD
	fd1      socketFD
	family   Family
	socktype Socktype
	protocol Protocol
	state    localSocketState
	name     atomic.Value
	peer     atomic.Value
	iovs     [][]byte
	addrBuf  [addrBufSize]byte
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
	s.fd0.close()
	s.fd1.close()
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
			return s.ns.bindInet4(s, bind)
		}
	case *SockaddrInet6:
		if s.family == INET6 {
			return s.ns.bindInet6(s, bind)
		}
	}
	return EAFNOSUPPORT
}

func (s *localSocket) bindAny() error {
	switch s.family {
	case INET:
		return s.ns.bindInet4(s, &sockaddrInet4Any)
	default:
		return s.ns.bindInet6(s, &sockaddrInet6Any)
	}
}

func (s *localSocket) bindLoopback() error {
	switch s.family {
	case INET:
		return s.ns.bindInet4(s, &sockaddrInet4Loopback)
	default:
		return s.ns.bindInet6(s, &sockaddrInet6Loopback)
	}
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

	s.state.set(listening)
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
		if err := s.bindLoopback(); err != nil {
			return err
		}
	}

	var peer *localSocket
	var err error
	switch a := addr.(type) {
	case *SockaddrInet4:
		peer, err = s.ns.lookupInet4(s, a)
	case *SockaddrInet6:
		peer, err = s.ns.lookupInet6(s, a)
	default:
		return EAFNOSUPPORT
	}
	if err != nil {
		return ECONNREFUSED
	}
	if peer.family != s.family {
		return ECONNREFUSED
	}
	if peer.socktype != s.socktype {
		return ECONNREFUSED
	}

	if s.socktype == DGRAM {
		s.peer.Store(addr)
		s.state.set(connected)
		return nil
	}

	peerFd := peer.fd1.acquire()
	if peerFd < 0 {
		return ECONNREFUSED
	}
	defer peer.fd1.release(peerFd)

	fd1 := s.fd1.acquire()
	if fd1 < 0 {
		return EBADF
	}
	defer s.fd1.release(fd1)

	addrBuf := encodeSockaddrAny(s.name.Load())
	// TODO: remove the heap allocation by implementing UnixRights to output to
	// a stack buffer.
	rights := unix.UnixRights(fd1)
	if err := unix.Sendmsg(peerFd, addrBuf[:], rights, nil, 0); err != nil {
		return ECONNREFUSED
	}

	s.fd1.close()
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

	socket := &localSocket{
		ns:       s.ns,
		family:   s.family,
		socktype: s.socktype,
		protocol: s.protocol,
		state:    bound | connected,
	}

	var oobn int
	var oobBuf [16]byte
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

	if s.state.is(listening) {
		return -1, 0, nil, EINVAL
	}
	if !s.state.is(bound) {
		if err := s.bindLoopback(); err != nil {
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
		n, _, rflags, addr, err := unix.RecvmsgBuffers(fd, iovs, nil, flags)
		if err == EINTR {
			if n == 0 {
				continue
			}
			err = nil
		}
		if s.socktype == DGRAM {
			addr = decodeSockaddr(s.addrBuf)
			n -= addrBufSize
		} else {
			addr = nil
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

	if s.state.is(listening) {
		return -1, EINVAL
	}
	if s.state.is(connected) && addr != nil {
		return -1, EISCONN
	}
	if !s.state.is(connected) && addr == nil {
		return -1, ENOTCONN
	}
	if !s.state.is(bound) {
		if err := s.bindLoopback(); err != nil {
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
	return ignoreEINTR(func() error { return unix.Shutdown(fd, how) })
}

func clearIOVecs(iovs [][]byte) {
	for i := range iovs {
		iovs[i] = nil
	}
}

func encodeSockaddrAny(addr any) (buf [addrBufSize]byte) {
	switch a := addr.(type) {
	case *SockaddrInet4:
		return encodeSockaddrInet4(a)
	case *SockaddrInet6:
		return encodeSockaddrInet6(a)
	default:
		return
	}
}

func encodeSockaddrInet4(addr *SockaddrInet4) (buf [addrBufSize]byte) {
	binary.LittleEndian.PutUint16(buf[0:2], uint16(INET))
	binary.LittleEndian.PutUint16(buf[2:4], uint16(addr.Port))
	*(*[4]byte)(buf[4:]) = addr.Addr
	return
}

func encodeSockaddrInet6(addr *SockaddrInet6) (buf [addrBufSize]byte) {
	binary.LittleEndian.PutUint16(buf[0:2], uint16(INET6))
	binary.LittleEndian.PutUint16(buf[2:4], uint16(addr.Port))
	*(*[16]byte)(buf[4:]) = addr.Addr
	return
}

func decodeSockaddr(buf [addrBufSize]byte) Sockaddr {
	switch Family(binary.LittleEndian.Uint16(buf[0:2])) {
	case INET:
		return &SockaddrInet4{
			Port: int(binary.LittleEndian.Uint16(buf[2:4])),
			Addr: ([4]byte)(buf[4:]),
		}
	default:
		return &SockaddrInet6{
			Port: int(binary.LittleEndian.Uint16(buf[2:4])),
			Addr: ([16]byte)(buf[4:]),
		}
	}
}

func iovecLen(iovs [][]byte) (n int) {
	for _, iov := range iovs {
		n += len(iov)
	}
	return n
}
