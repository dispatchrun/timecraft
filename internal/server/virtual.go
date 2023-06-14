package server

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

func (s *virtualSocketsSystem) SockBind(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	addr = s.translateAddressIn(addr)
	return s.System.SockBind(ctx, fd, addr)
}

func (s *virtualSocketsSystem) SockConnect(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	addr = s.translateAddressIn(addr)
	return s.System.SockConnect(ctx, fd, addr)
}

func (s *virtualSocketsSystem) SockLocalAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	addr, errno := s.System.SockLocalAddress(ctx, fd)
	return s.translateAddressOut(addr, errno)
}

func (s *virtualSocketsSystem) SockRemoteAddress(ctx context.Context, fd wasi.FD) (wasi.SocketAddress, wasi.Errno) {
	addr, errno := s.System.SockRemoteAddress(ctx, fd)
	return s.translateAddressOut(addr, errno)
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
