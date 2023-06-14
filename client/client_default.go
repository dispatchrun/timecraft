//go:build !wasip1

package client

import (
	"context"
	"crypto/tls"
	"net"
)

var dialContext = func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "unix", Socket)
}
