//go:build wasip1

package timecraft

import (
	"context"
	"crypto/tls"
	"net"

	"github.com/stealthrocket/net/wasip1"
)

func dialContext(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
	var d wasip1.Dialer
	return d.DialContext(ctx, "unix", timecraft.Socket)
}
