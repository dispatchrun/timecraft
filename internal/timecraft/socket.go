package timecraft

import (
	"context"

	"github.com/stealthrocket/wasi-go"
)

// NewVirtualSocketsSystem creates a wasi.System that translates unix socket
// addresses.
func NewVirtualSocketsSystem(system wasi.System, sockets map[string]string) wasi.System {
	in := map[wasi.UnixAddress]wasi.UnixAddress{}
	out := map[wasi.UnixAddress]wasi.UnixAddress{}

	for from, to := range sockets {
		fromAddr := wasi.UnixAddress{Name: from}
		toAddr := wasi.UnixAddress{Name: to}

		in[fromAddr] = toAddr
		out[toAddr] = fromAddr
	}

	return &virtualSocketsSystem{system, in, out}
}

type virtualSocketsSystem struct {
	wasi.System

	in  map[wasi.UnixAddress]wasi.UnixAddress
	out map[wasi.UnixAddress]wasi.UnixAddress
}

func (s *virtualSocketsSystem) SockAccept(ctx context.Context, fd wasi.FD, flags wasi.FDFlags) (wasi.FD, wasi.SocketAddress, wasi.SocketAddress, wasi.Errno) {
	fd, peer, addr, errno := s.System.SockAccept(ctx, fd, flags)
	peer, errno = s.translateAddressOut(peer, errno)
	addr, errno = s.translateAddressOut(addr, errno)
	return fd, peer, addr, errno
}

func (s *virtualSocketsSystem) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	addr = s.translateAddressIn(addr)
	local, errno := s.System.SockBind(ctx, fd, addr)
	return s.translateAddressOut(local, errno)
}

func (s *virtualSocketsSystem) SockConnect(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	addr = s.translateAddressIn(addr)
	peer, errno := s.System.SockConnect(ctx, fd, addr)
	return s.translateAddressOut(peer, errno)
}

func (s *virtualSocketsSystem) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	addr, errno := s.System.SockLocalAddress(ctx, fd)
	return s.translateAddressOut(addr, errno)
}

func (s *virtualSocketsSystem) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	addr, errno := s.System.SockRemoteAddress(ctx, fd)
	return s.translateAddressOut(addr, errno)
}

func (s *virtualSocketsSystem) SockRecvFrom(ctx context.Context, fd wasi.FD, iovecs []wasi.IOVec, flags wasi.RIFlags) (wasi.Size, wasi.ROFlags, wasi.SocketAddress, wasi.Errno) {
	size, oflags, peer, errno := s.System.SockRecvFrom(ctx, fd, iovecs, flags)
	peer, errno = s.translateAddressOut(peer, errno)
	return size, oflags, peer, errno
}

func (s *virtualSocketsSystem) translateAddressIn(addr wasi.SocketAddress) wasi.SocketAddress {
	unixAddr, ok := addr.(*wasi.UnixAddress)
	if ok {
		if replacement, ok := s.in[*unixAddr]; ok {
			addr = &replacement
		}
	}
	return addr
}

func (s *virtualSocketsSystem) translateAddressOut(addr wasi.SocketAddress, errno wasi.Errno) (wasi.SocketAddress, wasi.Errno) {
	if errno != wasi.ESUCCESS {
		return addr, errno
	}
	unixAddr, ok := addr.(*wasi.UnixAddress)
	if ok {
		if replacement, ok := s.out[*unixAddr]; ok {
			addr = &replacement
		}
	}
	return addr, wasi.ESUCCESS
}
