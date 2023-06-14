package server

import (
	"context"

	"github.com/stealthrocket/wasi-go"
)

// VirtualSocketsSystem wraps a wasi.System to translate unix socket addresses.
// TODO: SockBind, SockLocalAddress, SockRemoteAddress
type VirtualSocketsSystem struct {
	wasi.System

	Sockets map[wasi.UnixAddress]wasi.UnixAddress
}

func (s *VirtualSocketsSystem) SockConnect(ctx context.Context, fd wasi.FD, addr wasi.SocketAddress) (wasi.SocketAddress, wasi.Errno) {
	unixAddr, ok := addr.(*wasi.UnixAddress)
	if ok {
		if replacement, ok := s.Sockets[*unixAddr]; ok {
			addr = &replacement
		}
	}
	return s.System.SockConnect(ctx, fd, addr)
}
