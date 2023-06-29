//go:build wasip1

package timecraft

import (
	"context"
	"net"

	"github.com/stealthrocket/net/wasip1"
)

func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d wasip1.Dialer
	return d.DialContext(ctx, "tcp", TimecraftAddress)
}
