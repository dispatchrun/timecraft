//go:build wasip1

package main

import (
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"

	"github.com/bufbuild/connect-go"
	"github.com/planetscale/vtprotobuf/codec/grpc"
	"github.com/stealthrocket/net/wasip1"
	v1 "github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1"
	"github.com/stealthrocket/timecraft/gen/proto/go/timecraft/server/v1/serverv1connect"
	"golang.org/x/net/http2"
)

func main() {
	h2cClient := &http.Client{
		Transport: &http2.Transport{
			AllowHTTP: true,
			DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
				var d wasip1.Dialer
				return d.DialContext(ctx, "unix", "/tmp/timecraft.sock")
			},
			// TODO: timeouts/limits
		},
	}
	client := serverv1connect.NewTimecraftServiceClient(
		h2cClient,
		"http://localhost:8080/",
		connect.WithCodec(grpc.Codec{}),
	)
	req := connect.NewRequest(&v1.SubmitTaskRequest{
		Name: "foobar",
	})
	res, err := client.SubmitTask(context.Background(), req)
	if err != nil {
		panic(err)
	}
	log.Println(res.Msg.Code)
}
