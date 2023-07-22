package sandbox

import (
	"context"
	"time"
	"unsafe"

	"github.com/stealthrocket/timecraft/internal/htls"
	"github.com/stealthrocket/wasi-go"
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
	return wasi.MakeErrno(s.socket.SetNonBlock(flags.Has(wasi.NonBlock)))
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
	if err := socket.SetNonBlock(flags.Has(wasi.NonBlock)); err != nil {
		socket.Close()
		return nil, wasi.MakeErrno(err)
	}
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
	switch option {
	case wasi.ReuseAddress:
		return nil, wasi.ENOTSUP

	case wasi.QuerySocketType:
		switch s.socket.Type() {
		case STREAM:
			return wasi.IntValue(wasi.StreamSocket), wasi.ESUCCESS
		case DGRAM:
			return wasi.IntValue(wasi.DatagramSocket), wasi.ESUCCESS
		default:
			return nil, wasi.ENOTSUP
		}

	case wasi.QuerySocketError:
		return wasi.IntValue(wasi.MakeErrno(s.socket.Error())), wasi.ESUCCESS

	case wasi.DontRoute:
		return nil, wasi.ENOTSUP

	case wasi.Broadcast:
		return nil, wasi.ENOTSUP

	case wasi.SendBufferSize:
		v, err := s.socket.SendBuffer()
		return wasi.IntValue(v), wasi.MakeErrno(err)

	case wasi.RecvBufferSize:
		v, err := s.socket.RecvBuffer()
		return wasi.IntValue(v), wasi.MakeErrno(err)

	case wasi.KeepAlive:
		return nil, wasi.ENOTSUP

	case wasi.OOBInline:
		return nil, wasi.ENOTSUP

	case wasi.RecvLowWatermark:
		return nil, wasi.ENOTSUP

	case wasi.QueryAcceptConnections:
		listen, err := s.socket.IsListening()
		return boolToIntValue(listen), wasi.MakeErrno(err)

	case wasi.TcpNoDelay:
		nodelay, err := s.socket.TCPNoDelay()
		return boolToIntValue(nodelay), wasi.MakeErrno(err)

	case wasi.Linger:
		return nil, wasi.ENOTSUP

	case wasi.RecvTimeout:
		t, err := s.socket.RecvTimeout()
		return durationToTimeValue(t), wasi.MakeErrno(err)

	case wasi.SendTimeout:
		t, err := s.socket.SendTimeout()
		return durationToTimeValue(t), wasi.MakeErrno(err)

	case wasi.BindToDevice:
		return nil, wasi.ENOTSUP

	default:
		return nil, wasi.EINVAL
	}
}

func boolToIntValue(v bool) wasi.IntValue {
	if v {
		return 1
	}
	return 0
}

func durationToTimeValue(v time.Duration) wasi.TimeValue {
	return wasi.TimeValue(int64(v))
}

func (s *wasiSocket) SockSetOpt(ctx context.Context, option wasi.SocketOption, value wasi.SocketOptionValue) wasi.Errno {
	var (
		htlsServerName = wasi.MakeSocketOption(htls.Level, htls.ServerName)
	)

	var err error
	switch option {
	case htlsServerName:
		serverName, ok := value.(wasi.BytesValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetTLSServerName(string(serverName))
		}

	case wasi.ReuseAddress:
		return wasi.ENOTSUP

	case wasi.QuerySocketType:
		return wasi.ENOTSUP

	case wasi.QuerySocketError:
		return wasi.ENOTSUP

	case wasi.DontRoute:
		return wasi.ENOTSUP

	case wasi.Broadcast:
		return wasi.ENOTSUP

	case wasi.RecvBufferSize:
		size, ok := value.(wasi.IntValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetRecvBuffer(int(size))
		}

	case wasi.SendBufferSize:
		size, ok := value.(wasi.IntValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetSendBuffer(int(size))
		}

	case wasi.KeepAlive:
		return wasi.ENOTSUP

	case wasi.OOBInline:
		return wasi.ENOTSUP

	case wasi.RecvLowWatermark:
		return wasi.ENOTSUP

	case wasi.QueryAcceptConnections:
		return wasi.ENOTSUP

	case wasi.TcpNoDelay:
		nodelay, ok := value.(wasi.IntValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetTCPNoDelay(nodelay != 0)
		}

	case wasi.Linger:
		return wasi.ENOTSUP

	case wasi.RecvTimeout:
		timeout, ok := value.(wasi.TimeValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetRecvTimeout(time.Duration(timeout))
		}

	case wasi.SendTimeout:
		timeout, ok := value.(wasi.TimeValue)
		if !ok {
			err = EINVAL
		} else {
			err = s.socket.SetSendTimeout(time.Duration(timeout))
		}

	case wasi.BindToDevice:
		return wasi.ENOTSUP

	default:
		return wasi.EINVAL
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
