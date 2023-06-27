//go:build !wasip1

package timecraft

import (
	"context"
	"net"

	"github.com/stealthrocket/timecraft/sdk"
)

func dialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	var d net.Dialer
	return d.DialContext(ctx, "tcp", sdk.TimecraftAddress)
}
