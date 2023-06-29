//go:build !wasip1

package timecraft

import (
	"context"
	"net"
)

func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "tcp", TimecraftAddress)
}
