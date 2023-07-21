package sandbox

import (
	"context"
	"syscall"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/htls"
	"github.com/stealthrocket/wasi-go"
	"golang.org/x/sys/unix"
)

type wasiSocket struct {
	unimplementedFileMethods
	socket Socket
}

func (s *wasiSocket) Fd() uintptr {
	return s.socket.Fd()
}

func (s *wasiSocket) FDClose(ctx context.Context) wasi.Errno {
	return wasi.MakeErrno(s.socket.Close())
}

func (s *wasiSocket) FDRead(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	n, _, errno := s.SockRecv(ctx, iovs, 0)
	return n, errno
}

func (s *wasiSocket) FDWrite(ctx context.Context, iovs []wasi.IOVec) (wasi.Size, wasi.Errno) {
	return s.SockSendTo(ctx, iovs, 0, nil)
}

func (s *wasiSocket) FDStatSetFlags(ctx context.Context, flags wasi.FDFlags) wasi.Errno {
	s.socket.SetNonblock(flags.Has(wasi.NonBlock))
	return wasi.ESUCCESS
}

func (s *wasiSocket) FDFileStatGet(ctx context.Context) (wasi.FileStat, wasi.Errno) {
	var stat wasi.FileStat
	switch s.socket.Type() {
	case STREAM:
		stat.FileType = wasi.SocketStreamType
	case DGRAM:
		stat.FileType = wasi.SocketDGramType
	}
	return stat, wasi.ESUCCESS
}

func (s *wasiSocket) SockBind(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.MakeErrno(s.socket.Bind(toNetworkSockaddr(addr)))
}

func (s *wasiSocket) SockConnect(ctx context.Context, addr wasi.SocketAddress) wasi.Errno {
	return wasi.MakeErrno(s.socket.Connect(toNetworkSockaddr(addr)))
}

func (s *wasiSocket) SockListen(ctx context.Context, backlog int) wasi.Errno {
	return wasi.MakeErrno(s.socket.Listen(backlog))
}

func (s *wasiSocket) SockAccept(ctx context.Context, flags wasi.FDFlags) (File, wasi.Errno) {
	socket, _, err := s.socket.Accept()
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	socket.SetNonblock(flags.Has(wasi.NonBlock))
	return &wasiSocket{socket: socket}, wasi.ESUCCESS
}

func (s *wasiSocket) SockRecv(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.Errno) {
	n, rflags, _, err := s.socket.RecvFrom(makeIOVecs(iovs), recvFlags(flags))
	return wasi.Size(n), wasiROFlags(rflags), wasi.MakeErrno(err)
}

func (s *wasiSocket) SockRecvFrom(ctx context.Context, iovs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	n, rflags, addr, err := s.socket.RecvFrom(makeIOVecs(iovs), recvFlags(flags))
	return wasi.Size(n), wasiROFlags(rflags), toWasiSocketAddress(addr), wasi.MakeErrno(err)
}

func (s *wasiSocket) SockSend(ctx context.Context, iovs []wasi.IOVec, _ wasi.SIFlags) (wasi.Size, wasi.Errno) {
	n, err := s.socket.SendTo(makeIOVecs(iovs), nil, 0)
	return wasi.Size(n), wasi.MakeErrno(err)
}

func (s *wasiSocket) SockSendTo(ctx context.Context, iovs []wasi.IOVec, _ wasi.SIFlags, addr wasi.SocketAddress) (wasi.Size, wasi.Errno) {
	n, err := s.socket.SendTo(makeIOVecs(iovs), toNetworkSockaddr(addr), 0)
	return wasi.Size(n), wasi.MakeErrno(err)
}

func (s *wasiSocket) SockGetOpt(ctx context.Context, option wasi.SocketOption) (wasi.SocketOptionValue, wasi.Errno) {
	var netLevel int
	switch option.Level() {
	case wasi.SocketLevel:
		netLevel = SOL_SOCKET
	case wasi.TcpLevel:
		netLevel = IPPROTO_TCP
	default:
		return nil, wasi.EINVAL
	}

	var netOption int
	switch option {
	case wasi.ReuseAddress:
		netOption = SO_REUSEADDR
	case wasi.QuerySocketType:
		netOption = SO_TYPE
	case wasi.QuerySocketError:
		netOption = SO_ERROR
	case wasi.DontRoute:
		netOption = SO_DONTROUTE
	case wasi.Broadcast:
		netOption = SO_BROADCAST
	case wasi.SendBufferSize:
		netOption = SO_SNDBUF
	case wasi.RecvBufferSize:
		netOption = SO_RCVBUF
	case wasi.KeepAlive:
		netOption = SO_KEEPALIVE
	case wasi.OOBInline:
		netOption = SO_OOBINLINE
	case wasi.RecvLowWatermark:
		netOption = SO_RCVLOWAT
	case wasi.QueryAcceptConnections:
		netOption = SO_ACCEPTCONN
	case wasi.TcpNoDelay:
		netOption = TCP_NODELAY
	case wasi.Linger:
		return nil, wasi.ENOTSUP // TODO: implement SO_LINGER
	case wasi.RecvTimeout:
		netOption = SO_RCVTIMEO
	case wasi.SendTimeout:
		netOption = SO_SNDTIMEO
	case wasi.BindToDevice:
		return nil, wasi.ENOTSUP // TODO: implement SO_BINDTODEVICE
	default:
		return nil, wasi.EINVAL
	}

	switch option {
	case wasi.RecvTimeout, wasi.SendTimeout:
		t, err := s.socket.GetOptTimeval(netLevel, netOption)
		if err != nil {
			return nil, wasi.MakeErrno(err)
		}
		return wasi.TimeValue(t.Nano()), wasi.ESUCCESS
	}

	value, err := s.socket.GetOptInt(netLevel, netOption)
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}

	errno := wasi.ESUCCESS
	switch option {
	case wasi.QuerySocketType:
		switch value {
		case int(STREAM):
			value = int(wasi.StreamSocket)
		case int(DGRAM):
			value = int(wasi.DatagramSocket)
		default:
			value = -1
			errno = wasi.ENOTSUP
		}
	case wasi.QuerySocketError:
		value = int(wasi.MakeErrno(syscall.Errno(value)))
	}
	return wasi.IntValue(value), errno
}

func (s *wasiSocket) SockSetOpt(ctx context.Context, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	var (
		htlsServerName = wasi.MakeSocketOption(htls.Level, htls.ServerName)
	)

	var netLevel int
	switch level := option.Level(); level {
	case wasi.SocketLevel:
		netLevel = SOL_SOCKET
	case wasi.TcpLevel:
		netLevel = IPPROTO_TCP
	default:
		netLevel = int(level)
	}

	var netOption int
	switch option {
	case htlsServerName:
		netOption = htls.ServerName
	case wasi.ReuseAddress:
		netOption = SO_REUSEADDR
	case wasi.QuerySocketType:
		netOption = SO_TYPE
	case wasi.QuerySocketError:
		netOption = SO_ERROR
	case wasi.DontRoute:
		netOption = SO_DONTROUTE
	case wasi.Broadcast:
		netOption = SO_BROADCAST
	case wasi.SendBufferSize:
		netOption = SO_SNDBUF
	case wasi.RecvBufferSize:
		netOption = SO_RCVBUF
	case wasi.KeepAlive:
		netOption = SO_KEEPALIVE
	case wasi.OOBInline:
		netOption = SO_OOBINLINE
	case wasi.RecvLowWatermark:
		netOption = SO_RCVLOWAT
	case wasi.QueryAcceptConnections:
		netOption = SO_ACCEPTCONN
	case wasi.TcpNoDelay:
		netOption = TCP_NODELAY
	case wasi.Linger:
		return wasi.ENOTSUP // TODO: implement SO_LINGER
	case wasi.RecvTimeout:
		netOption = SO_RCVTIMEO
	case wasi.SendTimeout:
		netOption = SO_SNDTIMEO
	case wasi.BindToDevice:
		return wasi.ENOTSUP // TODO: implement SO_BINDTODEVICE
	default:
		return wasi.EINVAL
	}

	var strval wasi.BytesValue
	var intval wasi.IntValue
	var timeval wasi.TimeValue
	var ok bool

	switch option {
	case htlsServerName:
		strval, ok = value.(wasi.BytesValue)
	case wasi.RecvTimeout, wasi.SendTimeout:
		timeval, ok = value.(wasi.TimeValue)
	default:
		intval, ok = value.(wasi.IntValue)
	}
	if !ok {
		return wasi.EINVAL
	}

	var err error
	switch option {
	case wasi.RecvTimeout, wasi.SendTimeout:
		err = s.socket.SetOptTimeval(netLevel, netOption, unix.NsecToTimeval(int64(timeval)))
	case htlsServerName:
		err = s.socket.SetOptString(netLevel, netOption, string(strval))
	default:
		err = s.socket.SetOptInt(netLevel, netOption, int(intval))
	}
	return wasi.MakeErrno(err)
}

func (s *wasiSocket) SockLocalAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	name, err := s.socket.Name()
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return toWasiSocketAddress(name), wasi.ESUCCESS
}

func (s *wasiSocket) SockRemoteAddress(ctx context.Context) (wasi.SocketAddress, wasi.Errno) {
	peer, err := s.socket.Peer()
	if err != nil {
		return nil, wasi.MakeErrno(err)
	}
	return toWasiSocketAddress(peer), wasi.ESUCCESS
}

func (s *wasiSocket) SockShutdown(ctx context.Context, flags wasi.SDFlags) wasi.Errno {
	var shut int
	if flags.Has(wasi.ShutdownRD) {
		shut |= SHUTRD
	}
	if flags.Has(wasi.ShutdownWR) {
		shut |= SHUTWR
	}
	return wasi.MakeErrno(s.socket.Shutdown(shut))
}

func toNetworkSockaddr(addr wasi.SocketAddress) Sockaddr {
	switch a := addr.(type) {
	case *wasi.Inet4Address:
		return &SockaddrInet4{
			Port: a.Port,
			Addr: a.Addr,
		}
	case *wasi.Inet6Address:
		return &SockaddrInet6{
			Port: a.Port,
			Addr: a.Addr,
		}
	case *wasi.UnixAddress:
		return &SockaddrUnix{
			Name: a.Name,
		}
	default:
		return nil
	}
}

func toWasiSocketAddress(sa Sockaddr) wasi.SocketAddress {
	switch t := sa.(type) {
	case *SockaddrInet4:
		return &wasi.Inet4Address{
			Addr: t.Addr,
			Port: t.Port,
		}
	case *SockaddrInet6:
		return &wasi.Inet6Address{
			Addr: t.Addr,
			Port: t.Port,
		}
	case *SockaddrUnix:
		name := t.Name
		if len(name) == 0 {
			// For consistency across platforms, replace empty unix socket
			// addresses with @. On Linux, addresses where the first byte is
			// a null byte are considered abstract unix sockets, and the first
			// byte is replaced with @.
			name = "@"
		}
		return &wasi.UnixAddress{
			Name: name,
		}
	default:
		return nil
	}
}

func makeIOVecs(iovs []wasi.IOVec) [][]byte {
	return *(*[][]byte)(unsafe.Pointer(&iovs))
}

func recvFlags(riflags wasi.RIFlags) (flags int) {
	if riflags.Has(wasi.RecvPeek) {
		flags |= PEEK
	}
	if riflags.Has(wasi.RecvWaitAll) {
		flags |= WAITALL
	}
	return flags
}

func wasiROFlags(rflags int) (roflags wasi.ROFlags) {
	if (rflags & TRUNC) != 0 {
		roflags |= wasi.RecvDataTruncated
	}
	return roflags
}
