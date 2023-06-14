//go:build wasip1

package client

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/stealthrocket/net/wasip1"
)

var dialContext = func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
	var d wasip1.Dialer
	return d.DialContext(ctx, "unix", Socket)
}
